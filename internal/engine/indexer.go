package engine

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

type IndexerConfig struct {
	Store    store.Store
	Semantic *SemanticEngine
	Analyzer *AnalysisEngine
	Search   *SearchEngine
	ASTRules *ASTRuleEngine
	Logger   *slog.Logger
}

type IndexerPipeline struct {
	config IndexerConfig
}

func NewIndexerPipeline(cfg IndexerConfig) *IndexerPipeline {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &IndexerPipeline{config: cfg}
}

type indexCollector struct {
	items     []store.BatchItem
	ch        chan store.BatchItem
	ctx       context.Context
	store     store.Store
	done      chan struct{}
	err       error
	batchSize int
	semantic  *SemanticEngine
	semCh     chan semanticJob
	semWg     sync.WaitGroup
}

type semanticJob struct {
	symbolID int64
	text     string
}

func calculateBatchSize() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.Alloc > 1024*1024*1024 { // > 1GB
		return 100
	}
	if m.Alloc > 512*1024*1024 { // > 512MB
		return 250
	}
	return 500
}

func newIndexCollector(ctx context.Context, s store.Store, sem *SemanticEngine) *indexCollector {
	bs := calculateBatchSize()
	c := &indexCollector{
		items:     make([]store.BatchItem, 0, bs),
		ch:        make(chan store.BatchItem, 100),
		ctx:       ctx,
		store:     s,
		done:      make(chan struct{}),
		batchSize: bs,
		semantic:  sem,
	}

	if sem != nil {
		c.semCh = make(chan semanticJob, 1000)
		for i := 0; i < 4; i++ {
			c.semWg.Add(1)
			go func() {
				defer c.semWg.Done()
				for job := range c.semCh {
					vec, err := sem.GenerateEmbedding(ctx, job.text)
					if err == nil && len(vec) > 0 {
						if err := s.InsertSemanticVector(ctx, job.symbolID, vec); err != nil {
							fmt.Printf("failed to insert semantic vector symbolID=%d error=%v\n", job.symbolID, err)
						}
					} else if err != nil {
						fmt.Printf("failed to generate embedding error=%v\n", err)
					}
				}
			}()
		}
	}

	go c.run()
	return c
}

func (c *indexCollector) run() {
	defer close(c.done)
	for item := range c.ch {
		c.items = append(c.items, item)
		if len(c.items) >= c.batchSize {
			if err := c.flush(); err != nil && c.err == nil {
				c.err = err
			}
		}
	}
	if len(c.items) > 0 {
		if err := c.flush(); err != nil && c.err == nil {
			c.err = err
		}
	}
}

func (c *indexCollector) flush() error {
	if len(c.items) == 0 {
		return nil
	}
	err := c.store.SaveFileIndexBatch(c.ctx, c.items)
	if err == nil && c.semantic != nil && c.semCh != nil {
		for _, item := range c.items {
			for _, sym := range item.Symbols {
				text := sym.Name
				if sym.Doc != "" {
					text += "\n" + sym.Doc
				} else if sym.Signature != "" {
					text += "\n" + sym.Signature
				}
				c.semCh <- semanticJob{
					symbolID: int64(sym.ID),
					text:     text,
				}
			}
		}
	}
	c.items = c.items[:0]
	return err
}

func (c *indexCollector) Wait() error {
	<-c.done
	if c.semCh != nil {
		close(c.semCh)
		c.semWg.Wait()
	}
	return c.err
}

func (ip *IndexerPipeline) Run(ctx context.Context, path string) error {
	if ip.config.Store == nil {
		return fmt.Errorf("store not initialized")
	}

	validatedPath, err := utils.ValidatePath(path)
	if err != nil {
		return err
	}

	fi, err := os.Stat(validatedPath)
	if err != nil {
		return fmt.Errorf("error stating path: %w", err)
	}

	var parsedData map[string]*ParsedPackageData
	if fi.IsDir() {
		pd, loadErr := BatchLoadPackages(validatedPath)
		if loadErr != nil {
			ip.config.Logger.Warn("BatchLoadPackages failed, falling back to individual loading", "error", loadErr)
			parsedData = make(map[string]*ParsedPackageData)
		} else {
			parsedData = pd
		}
	} else {
		parsedData = make(map[string]*ParsedPackageData)
	}

	collector := newIndexCollector(ctx, ip.config.Store, ip.config.Semantic)

	maxWorkers := 4
	if runtime.NumCPU() < 4 {
		maxWorkers = runtime.NumCPU()
	}
	workerSem := make(chan struct{}, maxWorkers)

	var indexErr error
	if fi.IsDir() {
		_, indexErr = ip.indexDirectory(ctx, validatedPath, workerSem, collector, parsedData)
	} else {
		workerSem <- struct{}{}
		_, indexErr = ip.indexFile(ctx, validatedPath, workerSem, collector, parsedData)
		<-workerSem
	}

	close(collector.ch)
	collErr := collector.Wait()

	if indexErr != nil {
		return indexErr
	}
	if collErr != nil {
		return fmt.Errorf("collector failed: %w", collErr)
	}

	if err := ip.config.Store.RecomputeIndegrees(ctx); err != nil {
		ip.config.Logger.Error("failed to recompute indegrees", "error", err)
	}

	if ip.config.Analyzer != nil {
		if aErr := ip.config.Analyzer.ResolveInterfaces(ctx); aErr != nil {
			ip.config.Logger.Error("failed to resolve interfaces", "error", aErr)
		}
		if aErr := ip.config.Analyzer.ResolveCentrality(ctx); aErr != nil {
			ip.config.Logger.Error("failed to resolve centrality", "error", aErr)
		}
	}

	return err
}

func (ip *IndexerPipeline) indexDirectory(ctx context.Context, dir string, workerSem chan struct{}, collector *indexCollector, parsedData map[string]*ParsedPackageData) (string, error) {
	storedHash, _, err := ip.config.Store.GetDirectoryHash(ctx, dir)
	if err == nil && storedHash != "" {
		// Cache hit logic
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("error reading directory: %w", err)
	}

	var mu sync.Mutex
	var hashes []string
	var g errgroup.Group

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			g.Wait()
			return "", ctx.Err()
		default:
		}

		path := filepath.Join(dir, entry.Name())
		entry := entry

		if entry.IsDir() {
			if BlockedDirs[entry.Name()] {
				continue
			}

			g.Go(func() error {
				childHash, err := ip.indexDirectory(ctx, path, workerSem, collector, parsedData)
				if err != nil {
					ip.config.Logger.Error("failed to index directory", "path", path, "error", err)
					return err
				}
				if childHash != "" {
					mu.Lock()
					hashes = append(hashes, childHash)
					mu.Unlock()
				}
				return nil
			})
		} else {
			ext := filepath.Ext(path)
			if !SupportedExts[ext] {
				continue
			}

			info, err := entry.Info()
			if err != nil || info.Size() > 64*1024 { // skip files > 64KB
				continue
			}

			select {
			case <-ctx.Done():
				g.Wait()
				return "", ctx.Err()
			case workerSem <- struct{}{}:
			}

			g.Go(func() error {
				defer func() { <-workerSem }()

				childHash, err := ip.indexFile(ctx, path, workerSem, collector, parsedData)
				if err != nil {
					ip.config.Logger.Error("failed to index file", "path", path, "error", err)
					return err
				}
				if childHash != "" {
					mu.Lock()
					hashes = append(hashes, childHash)
					mu.Unlock()
				}
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return "", err
	}

	sort.Strings(hashes)
	dirHash := utils.StringHash(strings.Join(hashes, ""))

	if storedHash == dirHash {
		return storedHash, nil
	}

	err = ip.config.Store.SaveDirectoryHash(ctx, dir, dirHash, 0)
	if err != nil {
		return "", fmt.Errorf("failed to save directory hash: %w", err)
	}

	return dirHash, nil
}

func (ip *IndexerPipeline) indexFile(ctx context.Context, path string, workerSem chan struct{}, collector *indexCollector, parsedData map[string]*ParsedPackageData) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("error stating file: %w", err)
	}
	mtime := fi.ModTime().UnixNano()

	existingIdx, err := ip.config.Store.GetFileIndex(ctx, path)
	if err == nil && existingIdx != nil {
		if existingIdx.Mtime == int(mtime) {
			return existingIdx.Hash, nil // Truly unchanged, skip everything
		}
	}

	hash, err := utils.CalculateHash(path)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	if existingIdx != nil && existingIdx.Hash == hash {
		existingIdx.Mtime = int(mtime)
		err = ip.config.Store.SaveFileIndex(ctx, existingIdx)
		if err != nil {
			return "", fmt.Errorf("failed to update file index: %w", err)
		}
		return hash, nil
	}

	ip.config.Logger.Info("indexing file", "path", path)

	var itPointers iter.Seq[types.ASTPointer]
	var itCalls iter.Seq[types.ASTCall]
	var itFlows iter.Seq[types.DataFlow]
	var streamErr error

	if parsedData != nil && len(parsedData) > 0 && filepath.Ext(path) == ".go" {
		if pd, ok := parsedData[path]; ok {
			itPointers, itCalls, itFlows, streamErr = StreamSymbolsFromAST(ctx, pd.Fset, pd.File, pd.Pkg)
		} else {
			itPointers, itCalls, itFlows, streamErr = StreamSymbols(ctx, path)
			ip.config.Logger.Warn("file ignored by build tags", "path", path)
		}
	} else {
		itPointers, itCalls, itFlows, streamErr = StreamSymbols(ctx, path)
	}

	if streamErr != nil {
		return "", fmt.Errorf("parsing failed for %s: %w", path, streamErr)
	}

	batchItem := store.BatchItem{
		Index: &store.FileIndex{
			Path:    path,
			Mtime:   int(mtime),
			Hash:    hash,
			AstJson: "{}",
			Project: utils.GetRepoName(ctx),
		},
		Symbols:    []store.Symbol{},
		Calls:      []store.Call{},
		Flows:      []store.Flow{},
		Violations: []store.Violation{},
	}

	for ptr := range itPointers {
		batchItem.Symbols = append(batchItem.Symbols, store.Symbol{
			Name:           ptr.Name,
			Type:           ptr.Type,
			PackagePath:    ptr.PackagePath,
			ReceiverType:   ptr.ReceiverType,
			Signature:      ptr.Signature,
			Doc:            ptr.Doc,
			Path:           path,
			StartByte:      ptr.Range.Start,
			EndByte:        ptr.Range.End,
			StartLine:      ptr.StartLine,
			StartCol:       ptr.StartCol,
			EndLine:        ptr.EndLine,
			StructuralHash: ptr.StructuralHash,
		})

		if ip.config.Search != nil {
			docID := path + ":" + ptr.Name
			_ = ip.config.Search.IndexSymbol(docID, map[string]interface{}{
				"name": ptr.Name,
				"doc":  ptr.Doc,
				"path": path,
				"type": ptr.Type,
			})
		}
	}

	for c := range itCalls {
		batchItem.Calls = append(batchItem.Calls, store.Call{
			CallerName: c.CallerName,
			CalleeName: c.CalleeName,
			CalleePath: c.CalleePath,
			LinkType:   c.LinkType,
			Path:       path,
			Line:       c.Line,
		})
	}

	for f := range itFlows {
		batchItem.Flows = append(batchItem.Flows, store.Flow{
			Source: f.Source,
			Sink:   f.Sink,
			Type:   f.Type,
			Path:   path,
			Line:   f.Line,
		})
	}

	if ip.config.ASTRules != nil {
		violations, err := ip.config.ASTRules.Audit(ctx, path)
		if err == nil && len(violations) > 0 {
			for _, v := range violations {
				batchItem.Violations = append(batchItem.Violations, store.Violation{
					RuleID:    v.RuleID,
					FilePath:  v.File,
					Message:   v.Message,
					Severity:  v.Severity,
					StartLine: v.Range.Start.Line,
					StartCol:  v.Range.Start.Column,
					Text:      v.Text,
				})
			}
		}
	}

	select {
	case collector.ch <- batchItem:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	return hash, nil
}
