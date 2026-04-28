package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/tee"
	"github.com/Rogercode97/scouter/internal/telemetry"
	"github.com/Rogercode97/scouter/internal/utils"
)

// Pipeline orchestrates command execution, filtering, tracking, and tee.
type Pipeline struct {
	Registry     *filter.Registry
	Tracker      *telemetry.Tracker
	LSPManager   *lsp.Manager
	mu           sync.Mutex
	TeeConfig    tee.Config
	Verbose      int
	GainLevel    int // 0: compact, 1: signal (SNR), 2: raw
	UltraCompact bool
	Enrich       bool
}

// Run executes a command through the full pipeline.
func (p *Pipeline) Run(ctx context.Context, command string, args []string) int {
	subcommand := ""
	filterArgs := args
	if len(args) > 0 {
		subcommand = args[0]
		filterArgs = args[1:]
	}

	f := p.Registry.Match(command, subcommand, filterArgs)

	// Gain Control: RAW (2)
	if p.GainLevel == 2 {
		f = nil
	}

	if f == nil {
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: gain=raw or no filter for %q, passing through\n", command)
		}
		return p.Passthrough(ctx, command, args)
	}

	fullArgs := args
	finalArgs := args
	if injected, ok := p.Registry.ShouldInject(f, args); ok {
		finalArgs = injected
	}

	if p.Tracker != nil {
		p.Tracker.WarmUp(ctx)
	}

	timed := telemetry.Start(p.Tracker)

	result, err := Execute(ctx, command, finalArgs)
	if err != nil {
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: execute error: %v\n", err)
		}
		code, _ := Passthrough(ctx, command, fullArgs)
		return code
	}

	// DIVINE REDEMPTION: Passive Health Ingestion
	// If the command is 'go test' and we injected -json, ingest results to update fragility.
	if command == "go" && subcommand == "test" && strings.Contains(strings.Join(finalArgs, " "), "-json") {
		p.PassiveHealthIngest(ctx, result.Stdout)
	}

	filtered, filterErr := ApplyPipeline(ctx, f, result.Stdout, &LocalFileResolver{})
	if filterErr != nil {
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: filter error: %v\n", filterErr)
		}
		filtered = result.Stdout
	}

	// Gain Control: COMPACT (0)
	if p.GainLevel == 0 {
		lines := strings.Split(strings.TrimSpace(filtered), "\n")
		if len(lines) > 5 {
			// Reuse same signaling as head_tail action for consistency
			headN, tailN := 2, 2
			filtered = strings.Join(lines[:headN], "\n") + 
				fmt.Sprintf("\n... [scouter: truncated %d lines of noise in compact mode] ...\n", len(lines)-headN-tailN) + 
				strings.Join(lines[len(lines)-tailN:], "\n") + "\n"
		}
	}

	hint := tee.MaybeSave(result.Stdout, result.ExitCode, command, p.TeeConfig)

	fmt.Print(filtered)
	if hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	// DIVINE REDEMPTION: Universal Shadow Indexing (Asynchronous)
	go p.ShadowIndex(context.Background())

	inputTokens := utils.EstimateTokens(result.Stdout)
	if inputTokens > 0 {
		originalCmd := command + " " + strings.Join(fullArgs, " ")
		scouterCmd := command + " " + strings.Join(finalArgs, " ")
		outputTokens := utils.EstimateTokens(filtered)
		if err := timed.Track(ctx, originalCmd, scouterCmd, inputTokens, outputTokens); err != nil {
			fmt.Fprintf(os.Stderr, "scouter: tracking error: %v\n", err)
		}
	}

	return result.ExitCode
}

// PassiveHealthIngest parses test results from stdout and updates the risk map.
func (p *Pipeline) PassiveHealthIngest(ctx context.Context, output string) {
	cfg, _ := config.Load(ctx)
	db, err := store.New(ctx, cfg.Tracking.DBPath)
	if err != nil {
		return
	}
	defer db.Close()

	h := NewHealthEngine(db)
	if err := h.Ingest(ctx, strings.NewReader(output)); err != nil && p.Verbose > 0 {
		fmt.Fprintf(os.Stderr, "scouter: passive health ingestion failed: %v\n", err)
	}
}

// ShadowIndex re-indexes files modified by the current execution.
func (p *Pipeline) ShadowIndex(ctx context.Context) {
	changes, err := utils.GetLocalChanges(ctx)
	if err != nil || len(changes) == 0 {
		return
	}

	cfg, _ := config.Load(ctx)
	db, err := store.New(ctx, cfg.Tracking.DBPath)
	if err != nil {
		return
	}
	defer db.Close()

	cwd, _ := os.Getwd()
	indexedCount := 0

	for _, change := range changes {
		absPath := filepath.Join(cwd, change.Path)
		
		ext := filepath.Ext(absPath)
		if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".py" {
			continue
		}

		// Double check if hash changed before parsing (efficiency)
		hash, hashErr := utils.CalculateHash(absPath)
		if hashErr == nil {
			cached, err := db.GetFileIndex(ctx, absPath)
			if err == nil && cached.Hash == hash {
				continue // Already indexed
			}
		}

		p.mu.Lock()
		if p.LSPManager == nil {
			p.LSPManager = lsp.NewManager()
		}
		p.mu.Unlock()

		itPointers, itCalls, parseErr := StreamSymbols(ctx, absPath)
		if parseErr != nil {
			continue
		}

		err = db.WithTransaction(ctx, func(txCtx context.Context, tx store.Repository) error {
			stats, _ := os.Stat(absPath)
			
			// We still save the FileIndex with an empty ASTJSON or lazy-collected one
			// TASK: In next iteration, migrate FileIndex.ASTJSON to be optional
			if err := tx.SaveFileIndex(txCtx, &store.FileIndex{
				Path: absPath, Mtime: stats.ModTime().UnixNano(), Hash: hash,
			}); err != nil {
				return err
			}

			tx.ClearSymbols(txCtx, absPath)
			tx.ClearCalls(txCtx, absPath)
			
			for ptr := range itPointers {
				tx.SaveSymbol(txCtx, &store.Symbol{
					Name: ptr.Name, Type: ptr.Type, Doc: ptr.Doc, Path: absPath,
					StartByte: ptr.Range.Start, EndByte: ptr.Range.End,
					StartLine: ptr.StartLine, StartCol: ptr.StartCol, EndLine: ptr.EndLine,
				})
			}
			for c := range itCalls {
				tx.SaveCall(txCtx, store.Call{
					CallerName: c.CallerName, CalleeName: c.CalleeName, Path: absPath, Line: c.Line,
					CalleePath: c.CalleePath, LinkType: c.LinkType,
				})
			}
			return nil
		})

		if err == nil {
			indexedCount++
		}
	}

	if indexedCount > 0 && p.Verbose > 0 {
		fmt.Fprintf(os.Stderr, "scouter: shadow-indexed %d modified files\n", indexedCount)
	}

	// DIVINE REDEMPTION: Post-indexing resolution (Interfaces & Centrality)
	if indexedCount > 0 {
		if err := db.ResolveInterfaces(ctx); err != nil && p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: interface resolution failed: %v\n", err)
		}
		if err := db.ResolveCentrality(ctx); err != nil && p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: centrality resolution failed: %v\n", err)
		}
	}

	if p.Enrich && indexedCount > 0 {
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: performing semantic enrichment (Omniscience)...\n")
		}
		en := NewEnricher(db, p.LSPManager)
		if err := en.Enrich(ctx); err != nil && p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "scouter: enrichment failed: %v\n", err)
		}
	}
}

// Passthrough runs a command directly without filtering.
func (p *Pipeline) Passthrough(ctx context.Context, command string, args []string) int {
	code, err := Passthrough(ctx, command, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scouter: %v\n", err)
		return 1
	}
	
	go p.ShadowIndex(context.Background())
	return code
}

// ApplyPipeline executes filter actions sequentially.
func ApplyPipeline(ctx context.Context, f *filter.Filter, input string, resolver filter.SourceResolver) (string, error) {
	lines := strings.Split(input, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	result := filter.ActionResult{
		Lines:    lines,
		Metadata: make(map[string]any),
		Resolver: resolver,
	}

	for i, action := range f.Pipeline {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		fn, ok := filter.GetAction(action.ActionName)
		if !ok {
			return "", fmt.Errorf("unknown action %q at pipeline[%d]", action.ActionName, i)
		}

		var err error
		result, err = fn(ctx, result, action.Params)
		if err != nil {
			return "", fmt.Errorf("pipeline[%d] %s: %w", i, action.ActionName, err)
		}
	}

	return strings.Join(result.Lines, "\n") + "\n", nil
}
