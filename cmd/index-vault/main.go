package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"golang.org/x/sync/errgroup"
)

// IndexingJob represents a file that needs to be processed.
type IndexingJob struct {
	Path string
	Info os.FileInfo
	Hash string
}

// IndexingResult contains the results of processing a single file.
type IndexingResult struct {
	Path       string
	Info       os.FileInfo
	Symbols    []types.ASTPointer
	Calls      []types.ASTCall
	Hash       string
	Error      error
	IsManifest bool
	Deps       []types.Dependency
}

var (
	filesIndexed atomic.Int64
	symbolsTotal atomic.Int64
	callsTotal   atomic.Int64
	failedFiles  atomic.Int64
	filesSkipped atomic.Int64
	filesCleaned atomic.Int64
	visitedPaths sync.Map
)

var healthFlag = flag.Bool("health", false, "Read go test -json from stdin and ingest health data")

func main() {
	flag.Parse()
	startTime := time.Now()
	mainCtx := context.Background()
	
	cfg, err := config.Load(mainCtx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	dbPath := cfg.Tracking.DBPath

	s, err := store.New(mainCtx, dbPath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()

	if *healthFlag {
		h := engine.NewHealthEngine(s)
		if err := h.Ingest(mainCtx, os.Stdin); err != nil {
			log.Fatalf("Health ingestion failed: %v", err)
		}
		fmt.Println("Health data ingested successfully")
		return
	}

	workspacePath, _ := os.Getwd()
	fmt.Printf("--- Indexing Workspace: %s ---\n", workspacePath)

	// 0. Clear old dependencies
	if err := s.ClearDependencies(mainCtx); err != nil {
		log.Printf("Warning: failed to clear dependencies: %v", err)
	}

	g, groupCtx := errgroup.WithContext(mainCtx)
	jobs := make(chan IndexingJob, 100)
	results := make(chan IndexingResult, 100)

	// 1. Dispatcher: Walk the workspace and push jobs.
	g.Go(func() error {
		defer close(jobs)
		return filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() == ".git" {
				return filepath.SkipDir
			}

			ext := filepath.Ext(path)
			base := filepath.Base(path)

			isCode := !info.IsDir() && (ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" || ext == ".py")
			isManifest := base == "go.mod" || base == "package.json"

			if isCode || isManifest {
				visitedPaths.Store(path, true)

				var hash string
				if isCode {
					h, err := utils.CalculateHash(path)
					if err == nil {
						hash = h
						// Selective Indexing: Skip if hash matches.
						if idx, err := s.GetFileIndex(groupCtx, path); err == nil && idx.Hash == hash {
							filesSkipped.Add(1)
							return nil
						}
					}
				}

				select {
				case jobs <- IndexingJob{Path: path, Info: info, Hash: hash}:
				case <-groupCtx.Done():
					return groupCtx.Err()
				}
			}
			return nil
		})
	})

	// 2. Workers: Consume jobs, perform hashing/parsing, and push results.
	workerCount := runtime.NumCPU()
	if workerCount < 4 {
		workerCount = 4
	}
	if workerCount > 8 {
		workerCount = 8 // OOM Guard
	}

	for i := 0; i < workerCount; i++ {
		g.Go(func() error {
			for job := range jobs {
				res := IndexingResult{Path: job.Path, Info: job.Info}
				base := filepath.Base(job.Path)

				if base == "go.mod" {
					res.IsManifest = true
					deps, err := engine.ParseGoMod(groupCtx, job.Path)
					if err != nil {
						res.Error = err
					} else {
						res.Deps = deps
					}
				} else if base == "package.json" {
					res.IsManifest = true
					deps, err := engine.ParsePackageJSON(groupCtx, job.Path)
					if err != nil {
						res.Error = err
					} else {
						res.Deps = deps
					}
				} else {
					// Code file
					h := job.Hash
					if h == "" {
						var hashErr error
						h, hashErr = utils.CalculateHash(job.Path)
						if hashErr != nil {
							res.Error = hashErr
						}
					}

					if res.Error == nil {
						res.Hash = h
						syms, calls, parseErr := engine.ParseFile(groupCtx, job.Path)
						if parseErr != nil {
							res.Error = parseErr
						} else {
							res.Symbols = syms
							res.Calls = calls
						}
					}
				}

				select {
				case results <- res:
				case <-groupCtx.Done():
					return groupCtx.Err()
				}
			}
			return nil
		})
	}

	// Results Closer: Close results channel once dispatcher and workers are done.
	go func() {
		_ = g.Wait()
		close(results)
	}()

	// 3. Single Writer: Consume results and perform sequential DB updates.
	for res := range results {
		if res.Error != nil {
			fmt.Fprintf(os.Stderr, "  [Error] %s: %v\n", res.Path, res.Error)
			failedFiles.Add(1)
			continue
		}

		if res.IsManifest {
			fmt.Printf("Indexing Ecosystem: %s\n", res.Path)
			for _, d := range res.Deps {
				_ = s.SaveDependency(mainCtx, &d)
			}
			filesIndexed.Add(1)
			continue
		}

		// Save Code Index atomically.
		err = s.WithTransaction(mainCtx, func(tx store.Repository) error {
			if err := tx.SaveFileIndex(mainCtx, &store.FileIndex{
				Path:  res.Path,
				Mtime: res.Info.ModTime().UnixNano(),
				Hash:  res.Hash,
			}); err != nil {
				return err
			}

			if err := tx.ClearSymbols(mainCtx, res.Path); err != nil {
				return err
			}
			if err := tx.ClearCalls(mainCtx, res.Path); err != nil {
				return err
			}

			for _, sym := range res.Symbols {
				if err := tx.SaveSymbol(mainCtx, &store.Symbol{
					Name:      sym.Name,
					Type:      sym.Type,
					Path:      res.Path,
					Doc:       sym.Doc,
					StartByte: sym.Range.Start,
					EndByte:   sym.Range.End,
					StartLine: sym.StartLine,
					EndLine:   sym.EndLine,
				}); err != nil {
					return err
				}
			}

			for _, call := range res.Calls {
				if err := tx.SaveCall(mainCtx, store.Call{
					CallerName: call.CallerName,
					CalleeName: call.CalleeName,
					Path:       call.Path,
					Line:       call.Line,
				}); err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "  [Error] Transaction failed for %s: %v\n", res.Path, err)
			failedFiles.Add(1)
		} else {
			filesIndexed.Add(1)
			symbolsTotal.Add(int64(len(res.Symbols)))
			callsTotal.Add(int64(len(res.Calls)))
		}
	}

	// 4. Orphan Cleanup: Remove files from DB that no longer exist on disk.
	dbPaths, err := s.GetAllFilePaths(mainCtx)
	if err == nil {
		for _, path := range dbPaths {
			if _, found := visitedPaths.Load(path); !found {
				if err := s.DeleteFileIndex(mainCtx, path); err == nil {
					filesCleaned.Add(1)
				}
			}
		}
	}

	duration := time.Since(startTime)
	filesCount := filesIndexed.Load()
	failedCount := failedFiles.Load()
	skippedCount := filesSkipped.Load()
	cleanedCount := filesCleaned.Load()
	symbolsCount := symbolsTotal.Load()
	callsCount := callsTotal.Load()

	fmt.Println("\n--- Workspace Indexing Complete ---")
	fmt.Printf("Files:    %d (%d failed, %d skipped, %d cleaned)\n", filesCount+failedCount+skippedCount, failedCount, skippedCount, cleanedCount)
	fmt.Printf("Symbols:  %d\n", symbolsCount)
	fmt.Printf("Calls:    %d\n", callsCount)

	throughput := float64(filesCount+failedCount+skippedCount) / duration.Seconds()
	fmt.Printf("Time:     %.1fs (%.1f files/sec)\n", duration.Seconds(), throughput)
}
