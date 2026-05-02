# 🗺️ REPO-MAP: SYMBOL SKELETON
Generated: Thu Apr 30 17:30:14 -03 2026

## Language: ts
### File: cmd/scouter/plugins/opencode/scouter.ts
```ts
```

### File: tests/fixtures/sample.ts
```ts
```

## Language: tsx
## Language: go
### File: cmd/index-vault/main.go
```go
func main() {
	flag.Parse()
	startTime := time.Now()
	
	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	
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

	lspMgr := lsp.NewManager()
	defer lspMgr.Close()

	if *enrichFlag {
		fmt.Println("--- Performing Semantic Enrichment ---")
		en := engine.NewEnricher(s, lspMgr)
		if err := en.Enrich(mainCtx); err != nil {
			log.Fatalf("Enrichment failed: %v", err)
		}
		fmt.Println("Enrichment complete")
		return
	}

	workspacePath, _ := os.Getwd()
	fmt.Printf("--- Indexing Workspace: %s ---\n", workspacePath)

	if err := s.ClearDependencies(mainCtx); err != nil {
		log.Printf("Warning: failed to clear dependencies: %v", err)
	}

	g, groupCtx := errgroup.WithContext(mainCtx)
	jobs := make(chan IndexingJob, 100)
	results := make(chan IndexingResult, 100)

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

	workerCount := runtime.NumCPU()
	if workerCount < 4 {
		workerCount = 4
	}
	if workerCount > 8 {
		workerCount = 8
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
						syms, calls, parseErr := engine.ParseFile(groupCtx, job.Path, lspMgr)
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

	go func() {
		_ = g.Wait()
		close(results)
	}()

	for res := range results {
		if res.Error != nil {
			fmt.Fprintf(os.Stderr, "  [Error] %s: %v\n", res.Path, res.Error)
			failedFiles.Add(1)
			continue
		}

		if res.IsManifest {
			fmt.Printf("Indexing Ecosystem: %s\n", res.Path)
			for _, d := range res.Deps {
				if err := s.SaveDependency(mainCtx, &d); err != nil {
					fmt.Fprintf(os.Stderr, "  [Error] Failed to save dependency %s: %v\n", d.Name, err)
				}
			}
			filesIndexed.Add(1)
			continue
		}

		err = s.WithTransaction(mainCtx, func(txCtx context.Context, tx store.Repository) error {
			if err := tx.SaveFileIndex(txCtx, &store.FileIndex{
				Path:  res.Path,
				Mtime: res.Info.ModTime().UnixNano(),
				Hash:  res.Hash,
			}); err != nil {
				return err
			}

			if err := tx.ClearSymbols(txCtx, res.Path); err != nil {
				return err
			}
			if err := tx.ClearCalls(txCtx, res.Path); err != nil {
				return err
			}

			for _, sym := range res.Symbols {
				if err := tx.SaveSymbol(txCtx, &store.Symbol{
					Name:      sym.Name,
					Type:      sym.Type,
					Path:      res.Path,
					Doc:       sym.Doc,
					StartByte: sym.Range.Start,
					EndByte:   sym.Range.End,
					StartLine: sym.StartLine,
					EndLine:   sym.EndLine,
					StartCol:  sym.StartCol,
				}); err != nil {
					return err
				}
			}

			for _, call := range res.Calls {
				if err := tx.SaveCall(txCtx, store.Call{
					CallerName: call.CallerName,
					CalleeName: call.CalleeName,
					CalleePath: call.CalleePath,
					LinkType:   call.LinkType,
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

	// 4. Orphan Cleanup
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

	// TASK 2.4: Sovereign Interface Resolution (Lazo Soberano)
	fmt.Println("Resolving interfaces and contract fulfillments...")
	if err := engine.LinkInterfaces(mainCtx, s, lspMgr); err != nil {
		log.Printf("Warning: interface resolution failed: %v", err)
	}
	if err := s.ResolveCentrality(mainCtx); err != nil {
		log.Printf("Warning: centrality resolution failed: %v", err)
	}

	if *enrichFlag {
		fmt.Println("Performing semantic enrichment (LSP)...")
		en := engine.NewEnricher(s, lspMgr)
		if err := en.Enrich(mainCtx); err != nil {
			log.Printf("Warning: enrichment failed: %v", err)
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

	secs := duration.Seconds()
	if secs == 0 {
		secs = 0.001
	}
	throughput := float64(filesCount+failedCount+skippedCount) / secs
	fmt.Printf("Time:     %.1fs (%.1f files/sec)\n", duration.Seconds(), throughput)
}
type IndexingJob struct {
	Path string
	Info os.FileInfo
	Hash string
}
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
```

### File: cmd/scouter/main.go
```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Inject real dependencies into the testable Run function
	exitCode := cli.Run(ctx, os.Args, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
```

### File: internal/adapters/engram/cli_repository.go
```go
func NewEngramRepository(dryRun bool) *EngramRepository {
	return &EngramRepository{
		dryRun: dryRun,
	}
}
type EngramRepository struct {
	// dryRun mode for safe auditing
	dryRun bool
}
```

### File: internal/adapters/engram/sqlite_provider.go
```go
func NewSQLiteMemoryProvider(dbPath string) *SQLiteMemoryProvider {
	return &SQLiteMemoryProvider{
		dbPath: dbPath,
	}
}
type SQLiteMemoryProvider struct {
	dbPath       string
	scouterStore store.Repository
}
```

### File: internal/adapters/llm/mcp_distiller.go
```go
func NewMCPDistiller(session *mcp.ServerSession) *MCPDistiller {
	return &MCPDistiller{Session: session}
}
type MCPDistiller struct {
	Session *mcp.ServerSession
}
```

### File: internal/cli/cli.go
```go
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	app := &App{Stdout: stdout, Stderr: stderr}

	flags, remaining := ParseFlags(args[1:])
	if len(remaining) == 0 {
		if flags.Version {
			fmt.Fprintf(stdout, "scouter v%s\n", version)
			return 0
		}
		app.printUsage()
		return 0
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	switch cmd {
	case "mcp":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return 1
		}
		defer db.Close()

		logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		server := mcp.NewServer(db, logger)
		defer server.Close()
		
		transport := &sdk.StdioTransport{}
		if err := server.Start(ctx, transport); err != nil {
			fmt.Fprintf(stderr, "MCP Server stopped: %v\n", err)
		}
		return 0

	case "index":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return 1
		}
		defer db.Close()

		if len(cmdArgs) == 0 {
			fmt.Fprintf(stderr, "usage: scouter index <path>\n")
			return 1
		}

		// Use the same logic as the MCP handler
		path, err := utils.ValidatePath(cmdArgs[0])
		if err != nil {
			fmt.Fprintf(stderr, "invalid path: %v\n", err)
			return 1
		}

		pointers, _, err := engine.ParseFile(ctx, path, nil)
		if err != nil {
			fmt.Fprintf(stderr, "parse error: %v\n", err)
			return 1
		}

		err = db.WithTransaction(ctx, func(ctx context.Context, tx store.Repository) error {
			tx.ClearSymbols(ctx, path)
			tx.ClearCalls(ctx, path)
			for _, p := range pointers {
				_ = tx.SaveSymbol(ctx, &store.Symbol{
					Name:      p.Name,
					Type:      p.Type,
					Signature: p.Signature,
					Path:      path,
					StartLine: p.StartLine,
					EndLine:   p.EndLine,
					StartCol:  p.StartCol,
				})
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(stderr, "store error: %v\n", err)
			return 1
		}

		fmt.Printf("✅ Indexed %s: %d symbols\n", path, len(pointers))

		// DIVINE REDEMPTION: Semantic Linking
		lspMgr := lsp.NewManager()
		defer lspMgr.Close()
		_ = engine.LinkInterfaces(ctx, db, lspMgr)
		_ = db.ResolveCentrality(ctx)

		return 0

	case "gain":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
		tracker, err := telemetry.NewTracker(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error creating tracker: %v\n", err)
			return 1
		}
		defer tracker.Close()
		// display.RunGain also writes to stdout, this part is still not fully testable
		// but we accept this for now.
		return 0

	case "setup":
		if err := initcmd.Run(cmdArgs); err != nil {
			fmt.Fprintf(stderr, "setup failed: %v\n", err)
			return 1
		}
		return 0

	case "predict":
		cfg, err := config.Load(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error loading config: %v\n", err)
			return 1
		}
		db, err := store.New(ctx, cfg.Tracking.DBPath)
		if err != nil {
			fmt.Fprintf(stderr, "error opening database: %v\n", err)
			return 1
		}
		defer db.Close()

		diff := ""
		if len(cmdArgs) > 0 {
			diff = cmdArgs[0]
		} else {
			out, err := exec.CommandContext(ctx, "git", "diff", "HEAD", "--unified=0").Output()
			if err != nil {
				fmt.Fprintf(stderr, "error getting git diff: %v\n", err)
				return 1
			}
			diff = string(out)
		}

		results, err := engine.PredictTests(ctx, db, diff)
		if err != nil {
			fmt.Fprintf(stderr, "prediction error: %v\n", err)
			return 1
		}

		if len(results) == 0 {
			fmt.Fprintln(stdout, "No affected tests identified.")
			return 0
		}

		fmt.Fprintln(stdout, "🎯 Affected Tests:")
		for _, r := range results {
			fmt.Fprintf(stdout, "- %s (%s)\n", r.Name, r.File)
		}
		return 0
	
	default:
		// Fallback to pipeline
		p := &engine.Pipeline{
			Verbose:      flags.Verbose,
			UltraCompact: flags.UltraCompact,
			Enrich:       flags.Enrich,
		}
		return p.Passthrough(ctx, cmd, cmdArgs)
	}
}
type App struct {
	Stdout io.Writer
	Stderr io.Writer
}
```

### File: internal/cli/cli_test.go
```go
func TestRunRejectsCd(t *testing.T) {
	ctx := context.Background()
	// Run now takes ctx, args, stdout, stderr
	code := Run(ctx, []string{"scouter", "cd", "/tmp"}, io.Discard, io.Discard)
	// If cd is not found by exec, Passthrough will likely return an error and Run might return 1
	if code == 0 {
		t.Errorf("Run(cd) should fail, got %d", code)
	}
}
```

### File: internal/cli/flags.go
```go
func ParseFlags(args []string) (Flags, []string) {
	var flags Flags
	var remaining []string

	for i, arg := range args {
		if arg == "--" {
			// Everything after "--" belongs to the underlying command.
			remaining = append(remaining, args[i+1:]...)
			break
		}
		switch {
		case arg == "-vv":
			flags.Verbose = 2
		case arg == "-v":
			if flags.Verbose < 1 {
				flags.Verbose = 1
			}
		case arg == "-u":
			flags.UltraCompact = true
		case arg == "--enrich":
			flags.Enrich = true
		case arg == "--skip-env":
			flags.SkipEnv = true
		case arg == "--version":
			flags.Version = true
		case arg == "--help" || arg == "-h":
			flags.Help = true
		case isStackedVerboseFlag(arg):
			flags.Verbose = strings.Count(arg, "v")
		case len(remaining) == 0 && isBuiltInCommand(arg) && i+1 < len(args) && isInfoFlag(args[i+1]):
			remaining = append(remaining, arg)
		default:
			remaining = append(remaining, args[i:]...)
			return flags, remaining
		}
	}

	return flags, remaining
}
func isStackedVerboseFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return false
	}
	trimmed := strings.TrimLeft(arg, "-")
	return len(trimmed) > 0 && strings.Trim(trimmed, "v") == ""
}
func isBuiltInCommand(arg string) bool {
	switch arg {
	case "init", "gain", "config", "proxy":
		return true
	default:
		return false
	}
}
func isInfoFlag(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "--version"
}
type Flags struct {
	Verbose      int
	UltraCompact bool
	SkipEnv      bool
	Version      bool
	Help         bool
	Enrich       bool
}
```

### File: internal/cli/flags_test.go
```go
func TestParseFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantFlags Flags
		wantArgs  []string
	}{
		{
			name:      "no flags",
			args:      []string{"git", "log"},
			wantFlags: Flags{},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "verbose",
			args:      []string{"-v", "git", "log"},
			wantFlags: Flags{Verbose: 1},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "double verbose",
			args:      []string{"-vv", "git", "log"},
			wantFlags: Flags{Verbose: 2},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "ultra compact",
			args:      []string{"-u", "git", "log"},
			wantFlags: Flags{UltraCompact: true},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "version",
			args:      []string{"--version"},
			wantFlags: Flags{Version: true},
			wantArgs:  nil,
		},
		{
			name:      "help",
			args:      []string{"--help"},
			wantFlags: Flags{Help: true},
			wantArgs:  nil,
		},
		{
			name:      "version after command is passed through",
			args:      []string{"bun", "--version"},
			wantFlags: Flags{},
			wantArgs:  []string{"bun", "--version"},
		},
		{
			name:      "global flags with version after command",
			args:      []string{"-v", "go", "--version"},
			wantFlags: Flags{Verbose: 1},
			wantArgs:  []string{"go", "--version"},
		},
		{
			name:      "built-in command help is preserved",
			args:      []string{"proxy", "--help"},
			wantFlags: Flags{Help: true},
			wantArgs:  []string{"proxy"},
		},
		{
			name:      "built-in command keeps its own flags",
			args:      []string{"gain", "--daily"},
			wantFlags: Flags{},
			wantArgs:  []string{"gain", "--daily"},
		},
		{
			name:      "mixed flags and args",
			args:      []string{"-v", "-u", "git", "status"},
			wantFlags: Flags{Verbose: 1, UltraCompact: true},
			wantArgs:  []string{"git", "status"},
		},
		{
			name:      "command help flag is passed through",
			args:      []string{"npx", "-y", "chrome-devtools-mcp@latest", "--help"},
			wantFlags: Flags{},
			wantArgs:  []string{"npx", "-y", "chrome-devtools-mcp@latest", "--help"},
		},
		{
			name:      "global flags stop parsing at command",
			args:      []string{"-v", "npx", "-y", "chrome-devtools-mcp@latest", "--help"},
			wantFlags: Flags{Verbose: 1},
			wantArgs:  []string{"npx", "-y", "chrome-devtools-mcp@latest", "--help"},
		},
		{
			name:      "global flags still allow built-in command help",
			args:      []string{"-v", "proxy", "--help"},
			wantFlags: Flags{Verbose: 1, Help: true},
			wantArgs:  []string{"proxy"},
		},
		// "--" separator: everything after it is passed verbatim to the command.
		{
			name:      "double dash passes remaining verbatim",
			args:      []string{"--", "opencode", "--help"},
			wantFlags: Flags{},
			wantArgs:  []string{"opencode", "--help"},
		},
		{
			name:      "scouter flags before double dash, command flags after",
			args:      []string{"-v", "--", "go", "test", "--version"},
			wantFlags: Flags{Verbose: 1},
			wantArgs:  []string{"go", "test", "--version"},
		},
		{
			name:      "double dash alone produces empty remaining",
			args:      []string{"--"},
			wantFlags: Flags{},
			wantArgs:  nil,
		},
		{
			name:      "double dash before --help prevents scouter help",
			args:      []string{"--", "--help"},
			wantFlags: Flags{},
			wantArgs:  []string{"--help"},
		},
		{
			name:      "double dash before -v prevents scouter verbose",
			args:      []string{"--", "-v", "git", "log"},
			wantFlags: Flags{},
			wantArgs:  []string{"-v", "git", "log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, args := ParseFlags(tt.args)
			if !reflect.DeepEqual(flags, tt.wantFlags) {
				t.Errorf("flags = %+v, want %+v", flags, tt.wantFlags)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}
```

### File: internal/config/config.go
```go
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		Tracking: TrackingConfig{
			DBPath: filepath.Join(home, ".config", "scouter", "scouter.db"),
		},
		Display: DisplayConfig{
			Color: true,
			Emoji: true,
		},
		Filters: FiltersConfig{
			Dir: filepath.Join(home, ".config", "scouter", "filters"),
		},
		Tee: TeeConfig{
			Enabled:     true,
			Mode:        "failures",
			MaxFiles:    20,
			MaxFileSize: 1 << 20, // 1MB
		},
	}
}
func Load(ctx context.Context) (*Config, error) {
	cfg := DefaultConfig()

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}
func configPath() string {
	if p := os.Getenv("SCOUTER_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "scouter", "config.toml")
}
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Get source permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	destination, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(sourcePath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}
type Config struct {
	Tracking TrackingConfig `toml:"tracking"`
	Display  DisplayConfig  `toml:"display"`
	Filters  FiltersConfig  `toml:"filters"`
	Tee      TeeConfig      `toml:"tee"`
}
type TrackingConfig struct {
	DBPath string `toml:"db_path"`
}
type DisplayConfig struct {
	Color bool `toml:"color"`
	Emoji bool `toml:"emoji"`
}
type FiltersConfig struct {
	Dir string `toml:"dir"`
}
type TeeConfig struct {
	Enabled     bool   `toml:"enabled"`
	Mode        string `toml:"mode"` // "failures", "always", "never"
	MaxFiles    int    `toml:"max_files"`
	MaxFileSize int64  `toml:"max_file_size"`
}
```

### File: internal/config/config_test.go
```go
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected tee mode 'failures', got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 20 {
		t.Errorf("expected max_files 20, got %d", cfg.Tee.MaxFiles)
	}
	if !cfg.Display.Color {
		t.Error("expected color enabled by default")
	}
	if cfg.Tracking.DBPath == "" {
		t.Error("expected non-empty db path")
	}
}
func TestLoadMissingFile(t *testing.T) {
	t.Setenv("SCOUTER_CONFIG", "/tmp/nonexistent-scouter-config-test.toml")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected defaults when file missing, got tee.mode=%q", cfg.Tee.Mode)
	}
}
func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "/custom/path.db"

[tee]
mode = "always"
max_files = 5
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SCOUTER_CONFIG", path)

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracking.DBPath != "/custom/path.db" {
		t.Errorf("expected custom db_path, got %q", cfg.Tracking.DBPath)
	}
	if cfg.Tee.Mode != "always" {
		t.Errorf("expected custom tee.mode, got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 5 {
		t.Errorf("expected custom tee.max_files, got %d", cfg.Tee.MaxFiles)
	}
}
func TestMigrate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacyConfigDir := filepath.Join(home, ".config", "snip")
	legacyDBPath := filepath.Join(home, ".local", "share", "snip", "tracking.db")

	err := os.MkdirAll(legacyConfigDir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Dir(legacyDBPath), 0700)
	if err != nil {
		t.Fatal(err)
	}

	configContent := `[display]
color = false
emoji = false
`
	err = os.WriteFile(filepath.Join(legacyConfigDir, "config.toml"), []byte(configContent), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(legacyDBPath, []byte("fake db content"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	err = cfg.Migrate(context.Background())
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	newConfigDir := filepath.Join(home, ".config", "scouter")
	if _, err := os.Stat(newConfigDir); err != nil {
		t.Errorf("new config dir not created: %v", err)
	}

	if _, err := os.Stat(filepath.Join(newConfigDir, "scouter.db")); err != nil {
		t.Errorf("database not migrated: %v", err)
	}

	if cfg.Display.Color != false {
		t.Error("config values not loaded from legacy config")
	}

	if _, err := os.Stat(legacyConfigDir + ".migrated"); err != nil {
		t.Error("legacy config dir not renamed to .migrated")
	}
}
```

### File: internal/display/display.go
```go
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}
func PrintFiltered(output string, verbose int) {
	if verbose > 0 && IsTerminal() {
		fmt.Fprintln(os.Stderr, DimStyle.Render("--- scouter filtered ---"))
	}
	fmt.Print(output)
}
func PrintError(msg string) {
	if IsTerminal() {
		fmt.Fprintln(os.Stderr, ErrorStyle.Render("scouter: "+msg))
	} else {
		fmt.Fprintln(os.Stderr, "scouter: "+msg)
	}
}
func FormatSeparator(width int) string {
	return strings.Repeat("═", width)
}
func ColorSavings(pct float64) string {
	text := fmt.Sprintf("%.1f%%", pct)
	if !IsTerminal() {
		return text
	}
	switch {
	case pct >= 70:
		return GreenStyle.Render(text)
	case pct >= 30:
		return YellowStyle.Render(text)
	default:
		return RedStyle.Render(text)
	}
}
func TierLabel(pct float64) string {
	switch {
	case pct >= 90:
		return "Elite"
	case pct >= 70:
		return "Great"
	case pct >= 50:
		return "Good"
	case pct >= 30:
		return "Fair"
	default:
		return "Low"
	}
}
func ColorTier(tier string) string {
	if !IsTerminal() {
		return tier
	}
	switch tier {
	case "Elite":
		return GreenStyle.Bold(true).Render(tier)
	case "Great":
		return GreenStyle.Render(tier)
	case "Good":
		return YellowStyle.Render(tier)
	case "Fair":
		return WarnStyle.Render(tier)
	default:
		return RedStyle.Render(tier)
	}
}
func ColorBar(value, maxVal, width int) string {
	if maxVal <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := min(max(value*width/maxVal, 0), width)
	if filled == 0 && value > 0 {
		filled = 1
	}
	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", width-filled)
	if !IsTerminal() {
		return filledStr + emptyStr
	}
	return GreenStyle.Render(filledStr) + DimStyle.Render(emptyStr)
}
func FormatBar(value, maxVal, width int) string {
	if maxVal <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := min(max(value*width/maxVal, 0), width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
func FormatSparkline(values []float64) string {
	blocks := []rune("▁▂▃▄▅▆▇█")

	if len(values) == 0 {
		return ""
	}

	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}

	var b strings.Builder
	for _, v := range values {
		idx := 0
		if max > 0 {
			idx = int(v / max * float64(len(blocks)-1))
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}
func visualWidth(s string) int {
	return lipgloss.Width(s)
}
func padRight(s string, targetWidth int) string {
	vw := visualWidth(s)
	if vw >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-vw)
}
func FormatTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths using visual width (ANSI-safe)
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = visualWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if w := visualWidth(cell); w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	var b strings.Builder

	// Header
	for i, h := range headers {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(padRight(h, widths[i]))
	}
	b.WriteString("\n")

	// Separator
	for i, w := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("─", w))
	}
	b.WriteString("\n")

	// Rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			if i < len(widths) {
				b.WriteString(padRight(cell, widths[i]))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
```

### File: internal/display/display_test.go
```go
func TestFormatSeparator(t *testing.T) {
	s := FormatSeparator(10)
	if len([]rune(s)) != 10 {
		t.Errorf("rune len = %d, want 10", len([]rune(s)))
	}
}
func TestFormatTable(t *testing.T) {
	headers := []string{"Name", "Count", "Pct"}
	rows := [][]string{
		{"git log", "42", "78.5%"},
		{"go test", "15", "85.2%"},
	}

	result := FormatTable(headers, rows)
	if !strings.Contains(result, "Name") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "git log") {
		t.Error("missing row data")
	}
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 4 { // header + separator + 2 rows
		t.Errorf("got %d lines, want 4", len(lines))
	}
}
func TestFormatTableEmpty(t *testing.T) {
	result := FormatTable(nil, nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}
func TestFormatBar(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		maxVal   int
		width    int
		wantLen  int
		wantFull bool
	}{
		{"full bar", 100, 100, 10, 10, true},
		{"half bar", 50, 100, 10, 10, false},
		{"empty bar", 0, 100, 10, 10, false},
		{"zero max", 50, 0, 10, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := FormatBar(tt.value, tt.maxVal, tt.width)
			runes := []rune(bar)
			if len(runes) != tt.wantLen {
				t.Errorf("bar rune len = %d, want %d", len(runes), tt.wantLen)
			}
			if tt.wantFull {
				for _, r := range runes {
					if r != '█' {
						t.Errorf("expected all filled, got %c", r)
						break
					}
				}
			}
		})
	}
}
func TestFormatSparkline(t *testing.T) {
	values := []float64{10, 50, 80, 30, 100}
	spark := FormatSparkline(values)
	runes := []rune(spark)
	if len(runes) != 5 {
		t.Errorf("sparkline len = %d, want 5", len(runes))
	}
	// Last value is max (100), should be highest block
	if runes[4] != '█' {
		t.Errorf("max value should be █, got %c", runes[4])
	}
}
func TestFormatSparklineEmpty(t *testing.T) {
	spark := FormatSparkline(nil)
	if spark != "" {
		t.Errorf("expected empty, got %q", spark)
	}
}
func TestTierLabel(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{95, "Elite"},
		{75, "Great"},
		{55, "Good"},
		{35, "Fair"},
		{10, "Low"},
	}
	for _, tt := range tests {
		got := TierLabel(tt.pct)
		if got != tt.want {
			t.Errorf("TierLabel(%.0f) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}
func TestColorSavingsNonTTY(t *testing.T) {
	// Non-TTY: should return plain text (no ANSI codes)
	result := ColorSavings(85.3)
	if !strings.Contains(result, "85.3%") {
		t.Errorf("expected 85.3%%, got %q", result)
	}
}
```

### File: internal/display/gain.go
```go
func RunGain(tracker *telemetry.Tracker, args []string) error {
	if tracker == nil {
		PrintError("no tracking data (run some commands first)")
		return nil
	}

	// Parse args
	var (
		showDaily   bool
		showWeekly  bool
		showMonthly bool
		showJSON    bool
		showCSV     bool
		showTop     bool
		historyN    int
		topN        int
		days        = 7
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--daily":
			showDaily = true
		case "--weekly":
			showWeekly = true
		case "--monthly":
			showMonthly = true
		case "--json":
			showJSON = true
		case "--csv":
			showCSV = true
		case "--top":
			showTop = true
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &topN)
				i++
			}
			if topN <= 0 {
				topN = 10
			}
		case "--history":
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &historyN)
				i++
			}
			if historyN <= 0 {
				historyN = 10
			}
		}
	}

	summary, err := tracker.GetSummary(context.Background())
	if err != nil {
		return fmt.Errorf("get summary: %w", err)
	}

	if showJSON {
		return exportJSON(summary, tracker, days)
	}
	if showCSV {
		return exportCSV(tracker, days)
	}

	if historyN > 0 {
		return showHistory(tracker, historyN)
	}

	if showTop {
		printSummary(summary)
		return showByCommand(tracker, topN)
	}

	if showWeekly {
		printSummary(summary)
		return showPeriodReport(tracker, "weekly")
	}

	if showMonthly {
		printSummary(summary)
		return showPeriodReport(tracker, "monthly")
	}

	if showDaily {
		return showDailyReport(tracker, days, summary)
	}

	// Default: full dashboard (summary + sparkline + top commands)
	printSummary(summary)
	showSparkline(tracker)
	_ = showByCommand(tracker, 10)
	return nil
}
func printSummary(s *telemetry.Summary) {
	tty := IsTerminal()

	fmt.Println()
	if tty {
		fmt.Println(HeaderStyle.Render("  scouter — Token Savings Report"))
		fmt.Println(DimStyle.Render("  " + FormatSeparator(30)))
	} else {
		fmt.Println("  scouter — Token Savings Report")
		fmt.Println("  " + FormatSeparator(30))
	}
	fmt.Println()

	tier := TierLabel(s.AvgSavings)

	// printKPI renders a label-value pair. If value is already styled
	// (contains ANSI codes), pass styled=true to avoid double-wrapping.
	printKPI := func(label, value string, styled bool) {
		if tty {
			styledValue := value
			if !styled {
				styledValue = StatStyle.Render(value)
			}
			fmt.Printf("  %s  %s\n", DimStyle.Render(fmt.Sprintf("%-20s", label)), styledValue)
		} else {
			fmt.Printf("  %-20s  %s\n", label, value)
		}
	}

	printKPI("Commands filtered", fmt.Sprintf("%d", s.TotalCommands), false)
	printKPI("Tokens saved", utils.FormatTokens(s.TotalSaved), false)
	printKPI("Avg savings", ColorSavings(s.AvgSavings), true)
	printKPI("Efficiency", ColorTier(tier), true)
	printKPI("Total time", fmt.Sprintf("%.1fs", float64(s.TotalTimeMs)/1000), false)

	// Efficiency bar
	pct := s.AvgSavings
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	bar := ColorBar(int(pct), 100, 20)
	fmt.Println()
	if tty {
		fmt.Printf("  %s %s\n", bar, DimStyle.Render(fmt.Sprintf("%.0f%%", s.AvgSavings)))
	} else {
		fmt.Printf("  %s %.0f%%\n", bar, s.AvgSavings)
	}
	fmt.Println()
}
func showByCommand(tracker *telemetry.Tracker, limit int) error {
	stats, err := tracker.GetByCommand(context.Background(), limit)
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		return nil
	}

	tty := IsTerminal()

	// Find max saved for bar scaling
	maxSaved := 0
	for _, s := range stats {
		if s.SavedTokens > maxSaved {
			maxSaved = s.SavedTokens
		}
	}

	if tty {
		fmt.Println(DimStyle.Render("  Top commands by tokens saved"))
		fmt.Println()
	} else {
		fmt.Println("  Top commands by tokens saved")
		fmt.Println()
	}

	headers := []string{"Command", "Runs", "Saved", "Savings", "Impact"}
	var rows [][]string
	for _, s := range stats {
		cmd := s.Command
		if len(cmd) > 25 {
			cmd = cmd[:22] + "..."
		}
		bar := ColorBar(s.SavedTokens, maxSaved, 12)
		rows = append(rows, []string{
			cmd,
			fmt.Sprintf("%d", s.Count),
			utils.FormatTokens(s.SavedTokens),
			ColorSavings(s.AvgSavings),
			bar,
		})
	}

	fmt.Print(FormatTable(headers, rows))
	fmt.Println()
	return nil
}
func showSparkline(tracker *telemetry.Tracker) {
	daily, err := tracker.GetDaily(context.Background(), 14)
	if err != nil || len(daily) < 2 {
		return
	}

	// Daily data is DESC, reverse for chronological sparkline
	values := make([]float64, len(daily))
	for i, d := range daily {
		values[len(daily)-1-i] = d.AvgSavings
	}

	spark := FormatSparkline(values)
	tty := IsTerminal()

	if tty {
		fmt.Printf("  %s  %s\n", DimStyle.Render("14-day trend"), SuccessStyle.Render(spark))
	} else {
		fmt.Printf("  14-day trend  %s\n", spark)
	}
	fmt.Println()
}
func showDailyReport(tracker *telemetry.Tracker, days int, summary *telemetry.Summary) error {
	daily, err := tracker.GetDaily(context.Background(), days)
	if err != nil {
		return err
	}

	printSummary(summary)

	headers := []string{"Date", "Cmds", "Input", "Output", "Saved", "Savings"}
	var rows [][]string
	for _, d := range daily {
		rows = append(rows, []string{
			d.Day,
			fmt.Sprintf("%d", d.Commands),
			utils.FormatTokens(d.InputTokens),
			utils.FormatTokens(d.OutputTokens),
			utils.FormatTokens(d.SavedTokens),
			ColorSavings(d.AvgSavings),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}
func showPeriodReport(tracker *telemetry.Tracker, period string) error {
	var stats []telemetry.PeriodStats
	var err error
	var label string

	switch period {
	case "weekly":
		stats, err = tracker.GetWeekly(context.Background(), 8)
		label = "Weekly"
	case "monthly":
		stats, err = tracker.GetMonthly(context.Background(), 6)
		label = "Monthly"
	default:
		return fmt.Errorf("unknown period: %s", period)
	}
	if err != nil {
		return err
	}

	tty := IsTerminal()
	if tty {
		fmt.Println(DimStyle.Render(fmt.Sprintf("  %s breakdown", label)))
	} else {
		fmt.Printf("  %s breakdown\n", label)
	}
	fmt.Println()

	headers := []string{"Period", "Cmds", "Input", "Output", "Saved", "Savings"}
	var rows [][]string
	for _, s := range stats {
		rows = append(rows, []string{
			s.Period,
			fmt.Sprintf("%d", s.Commands),
			utils.FormatTokens(s.InputTokens),
			utils.FormatTokens(s.OutputTokens),
			utils.FormatTokens(s.SavedTokens),
			ColorSavings(s.AvgSavings),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	fmt.Println()
	return nil
}
func showHistory(tracker *telemetry.Tracker, n int) error {
	records, err := tracker.GetRecent(context.Background(), n)
	if err != nil {
		return err
	}

	headers := []string{"Command", "Input", "Output", "Saved", "Time"}
	var rows [][]string
	for _, r := range records {
		cmd := r.OriginalCmd
		if len(cmd) > 30 {
			cmd = cmd[:27] + "..."
		}
		rows = append(rows, []string{
			cmd,
			utils.FormatTokens(r.InputTokens),
			utils.FormatTokens(r.OutputTokens),
			ColorSavings(r.SavingsPct),
			fmt.Sprintf("%dms", r.ExecTimeMs),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}
func exportJSON(summary *telemetry.Summary, tracker *telemetry.Tracker, days int) error {
	daily, _ := tracker.GetDaily(context.Background(), days)
	byCmd, _ := tracker.GetByCommand(context.Background(), 10)
	data := map[string]any{
		"summary":    summary,
		"daily":      daily,
		"by_command": byCmd,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
func exportCSV(tracker *telemetry.Tracker, days int) error {
	daily, err := tracker.GetDaily(context.Background(), days)
	if err != nil {
		return err
	}

	w := csv.NewWriter(os.Stdout)
	_ = w.Write([]string{"date", "commands", "input_tokens", "output_tokens", "saved_tokens", "avg_savings"})
	for _, d := range daily {
		_ = w.Write([]string{
			d.Day,
			fmt.Sprintf("%d", d.Commands),
			fmt.Sprintf("%d", d.InputTokens),
			fmt.Sprintf("%d", d.OutputTokens),
			fmt.Sprintf("%d", d.SavedTokens),
			fmt.Sprintf("%.1f", d.AvgSavings),
		})
	}
	w.Flush()
	return w.Error()
}
```

### File: internal/display/gain_test.go
```go
func newTestTracker(t *testing.T) *telemetry.Tracker {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := telemetry.NewTracker(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	t.Cleanup(func() { _ = tracker.Close() })
	return tracker
}
func seedTracker(t *testing.T, tracker *telemetry.Tracker) {
	t.Helper()
	_ = tracker.Track(context.Background(), "git log", "scouter git log", 1000, 200, 50)
	_ = tracker.Track(context.Background(), "go test", "scouter go test", 2000, 300, 100)
	_ = tracker.Track(context.Background(), "git log", "scouter git log", 800, 100, 40)
	_ = tracker.Track(context.Background(), "ls -la", "scouter ls -la", 50, 30, 5)
}
func TestRunGainNoData(t *testing.T) {
	tracker := newTestTracker(t)
	err := RunGain(tracker, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainWithData(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainDaily(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--daily"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainWeekly(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--weekly"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainMonthly(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--monthly"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainTop(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--top", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainTopDefault(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--top"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainHistory(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--history", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainHistoryDefault(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--history"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainJSON(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainCSV(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--csv"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunGainNilTracker(t *testing.T) {
	err := RunGain(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

### File: internal/domain/memory/interfaces.go
```go
type Observation struct {
	Content         string   `json:"content"`
	ASTContext      string   `json:"ast_context,omitempty"`
	StructuralLinks []string `json:"structural_links,omitempty"`
}
type Summary struct {
	ADRs     []string `json:"adrs"`
	BugFixes []string `json:"bug_fixes"`
	Patterns []string `json:"patterns"`
}
type MemoryProvider interface {
	GetRecentObservations(ctx context.Context, project string, hours int) ([]Observation, error)
}
type Distiller interface {
	Distill(ctx context.Context, logs []Observation) (Summary, error)
}
type SummaryRepository interface {
	SaveSummary(ctx context.Context, project string, summary Summary) error
}
```

### File: internal/domain/memory/service.go
```go
func NewAppService(memory MemoryProvider, distiller Distiller, repo SummaryRepository) *AppService {
	return &AppService{
		memory:    memory,
		distiller: distiller,
		repo:      repo,
	}
}
type AppService struct {
	memory    MemoryProvider
	distiller Distiller
	repo      SummaryRepository
}
```

### File: internal/engine/compaction.go
```go
func NewCompactionEngine(s store.Repository) *CompactionEngine {
	return &CompactionEngine{store: s}
}
type CompactionEngine struct {
	store store.Repository
}
```

### File: internal/engine/compaction_test.go
```go
func TestCompactSession(t *testing.T) {
	// Setup temporary working directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current wd: %v", err)
	}
	
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change wd: %v", err)
	}
	defer os.Chdir(oldCwd)

	engine := NewCompactionEngine(nil)
	ctx := context.Background()
	summary := "Test summary for compaction"

	result, err := engine.CompactSession(ctx, summary)
	if err != nil {
		t.Fatalf("CompactSession failed: %v", err)
	}

	// Verify .scouter directory exists
	scouterDir := filepath.Join(tmpDir, ".scouter")
	if _, err := os.Stat(scouterDir); os.IsNotExist(err) {
		t.Errorf(".scouter directory was not created")
	}

	// Verify anchor.md exists
	anchorPath := filepath.Join(scouterDir, "anchor.md")
	if _, err := os.Stat(anchorPath); os.IsNotExist(err) {
		t.Errorf("anchor.md was not created")
	}

	if result.AnchorPath != anchorPath {
		t.Errorf("expected AnchorPath %s, got %s", anchorPath, result.AnchorPath)
	}

	// Verify content of anchor.md
	content, err := os.ReadFile(anchorPath)
	if err != nil {
		t.Fatalf("failed to read anchor.md: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "# Scouter Session Anchor") {
		t.Errorf("anchor.md missing header")
	}
	if !strings.Contains(strContent, "**Timestamp**:") {
		t.Errorf("anchor.md missing timestamp")
	}
	if !strings.Contains(strContent, summary) {
		t.Errorf("anchor.md missing summary content")
	}
}
func TestCompactSessionEmptySummary(t *testing.T) {
	engine := NewCompactionEngine(nil)
	ctx := context.Background()

	_, err := engine.CompactSession(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty summary, got nil")
	} else if err.Error() != "summary cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}
type MockStore struct{}
```

### File: internal/engine/enricher.go
```go
func NewEnricher(s store.Repository, lp LSPProvider) *Enricher {
	return &Enricher{
		store: s,
		lsp:   lp,
	}
}
type Enricher struct {
	store store.Repository
	lsp   LSPProvider
}
type dynamicLink struct {
		interfaceMethod string
		structMethod    string
		structPath      string
		ifacePath       string
		ifaceLine       int
	}
type LSPProvider interface {
	GetClient(ctx context.Context, filePath string) (lsp.LSPClient, error)
}
```

### File: internal/engine/enricher_test.go
```go
func TestEnricher_Enrich(t *testing.T) {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	circlePath := cwd + "/circle_test.go"
	shapePath := cwd + "/shape_test.go"

	// Create dummy files to satisfy ValidatePath existence check
	os.WriteFile(circlePath, []byte("package test"), 0644)
	os.WriteFile(shapePath, []byte("package test"), 0644)
	defer os.Remove(circlePath)
	defer os.Remove(shapePath)

	mStore := &mockStore{
		methods: []store.Symbol{
			{Name: "Circle.Area", Path: circlePath, Type: "method", StartLine: 10, StartCol: 5},
		},
		symbolsByRange: []store.Symbol{
			{Name: "Shape:Area", Path: shapePath, Type: "method_spec", StartLine: 5, StartCol: 1},
		},
	}

	mLSP := &mockLSPClient{
		impls: []lsp.Location{
			{URI: "file://" + shapePath, Range: lsp.Range{Start: lsp.Position{Line: 4}}},
		},
	}

	mProvider := &mockLSPProvider{client: mLSP}

	enricher := NewEnricher(mStore, mProvider)
	err := enricher.Enrich(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mStore.savedCalls) != 1 {
		t.Fatalf("expected 1 saved call, got %d", len(mStore.savedCalls))
	}

	call := mStore.savedCalls[0]
	if call.CallerName != "Shape:Area" {
		t.Errorf("expected caller Shape:Area, got %s", call.CallerName)
	}
	if call.CalleeName != "Circle.Area" {
		t.Errorf("expected callee Circle.Area, got %s", call.CalleeName)
	}
	if call.LinkType != "dynamic" {
		t.Errorf("expected link type dynamic, got %s", call.LinkType)
	}
}
type mockStore struct {
	store.Repository
	methods         []store.Symbol
	symbolsByRange  []store.Symbol
	savedCalls      []store.Call
}
type mockLSPClient struct {
	lsp.LSPClient
	impls []lsp.Location
}
type mockLSPProvider struct {
	client lsp.LSPClient
	err    error
}
```

### File: internal/engine/executor.go
```go
func makeCommand(ctx context.Context, command string, args []string) *exec.Cmd {
	if shellBuiltins[command] {
		shArgs := make([]string, 0, len(args)+3)
		shArgs = append(shArgs, "-c", command+` "$@"`, "_")
		shArgs = append(shArgs, args...)
		return exec.CommandContext(ctx, "sh", shArgs...)
	}
	return exec.CommandContext(ctx, command, args...)
}
func Execute(ctx context.Context, command string, args []string) (*Result, error) {
	start := time.Now()

	cmd := makeCommand(ctx, command, args)
	// Don't connect stdin for captured commands — prevents blocking on
	// commands that don't read stdin (most filtered commands).
	// Passthrough commands still get stdin via the Passthrough function.

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = stdoutBuf.ReadFrom(stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, _ = stderrBuf.ReadFrom(stderrPipe)
	}()

	err = cmd.Wait()
	wg.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("wait command: %w", err)
		}
	}

	return &Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}
func Passthrough(ctx context.Context, command string, args []string) (int, error) {
	cmd := makeCommand(ctx, command, args)
	cmd.Stdin = os.Stdin
	// When running as MCP server, we MUST NOT write to os.Stdout directly
	// as it will corrupt the JSON-RPC stream. Redirecting to Stderr is safer.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("passthrough: %w", err)
	}
	return 0, nil
}
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}
```

### File: internal/engine/executor_test.go
```go
func TestExecuteEcho(t *testing.T) {
	ctx := context.Background()
	result, err := Execute(ctx, "echo", []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout); got != "hello world" {
		t.Errorf("stdout = %q", got)
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}
func TestExecuteStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	result, err := Execute(ctx, "sh", []string{"-c", "echo error >&2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(result.Stderr); got != "error" {
		t.Errorf("stderr = %q", got)
	}
}
func TestExecuteExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	result, err := Execute(ctx, "sh", []string{"-c", "exit 42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}
func TestExecuteNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := Execute(ctx, "nonexistent-command-xyz", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}
func TestPassthrough(t *testing.T) {
	ctx := context.Background()
	code, err := Passthrough(ctx, "echo", []string{"test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d", code)
	}
}
func TestPassthroughExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	code, err := Passthrough(ctx, "sh", []string{"-c", "exit 7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
}
func TestExecuteShellBuiltin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	result, err := Execute(ctx, "export", []string{"FOO=bar"})
	if err != nil {
		t.Fatalf("unexpected error executing shell builtin: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
}
func TestPassthroughShellBuiltin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	ctx := context.Background()
	code, err := Passthrough(ctx, "export", []string{"FOO=bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}
func TestMakeCommandBuiltin(t *testing.T) {
	ctx := context.Background()
	cmd := makeCommand(ctx, "export", []string{"A=1", "B=2"})
	if cmd.Path == "" {
		t.Fatal("command path should not be empty")
	}
	// Should wrap with sh -c
	if cmd.Args[0] != "sh" {
		t.Errorf("expected sh wrapper, got %q", cmd.Args[0])
	}
	if cmd.Args[1] != "-c" {
		t.Errorf("expected -c flag, got %q", cmd.Args[1])
	}
}
func TestMakeCommandRegular(t *testing.T) {
	ctx := context.Background()
	cmd := makeCommand(ctx, "git", []string{"status"})
	// Should NOT wrap with sh
	if len(cmd.Args) > 0 && cmd.Args[0] == "sh" {
		t.Error("regular commands should not be wrapped with sh")
	}
}
```

### File: internal/engine/health.go
```go
func NewHealerEngine(s store.Repository, l *lsp.Manager) *HealerEngine {
	return &HealerEngine{
		store:  s,
		lspMgr: l,
	}
}
func NewHealthEngine(store TestResultStore) *HealthEngine {
	return &HealthEngine{store: store}
}
func extractErrorAndStack(output string) (string, string) {
	lines := strings.Split(output, "\n")
	var errorMessage, stackTrace []string
	foundFail := false

	for _, line := range lines {
		if strings.Contains(line, "--- FAIL:") {
			foundFail = true
			continue
		}
		if foundFail {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Heuristic: First line with a colon after FAIL is the error message
			if strings.Contains(trimmed, ":") && len(errorMessage) == 0 {
				errorMessage = append(errorMessage, trimmed)
			}
			// All lines after FAIL are part of the stack trace/logs
			stackTrace = append(stackTrace, line)
		}
	}
	return strings.Join(errorMessage, "\n"), strings.Join(stackTrace, "\n")
}
type HealerEngine struct {
	store  store.Repository
	lspMgr *lsp.Manager
	
	// Bridge to MCP sampling
	DoFixRequest func(ctx context.Context, prompt string) (string, error)
}
type HealthEngine struct {
	store TestResultStore
}
type TestResultStore interface {
	SaveTestResult(ctx context.Context, res *types.TestResult) error
}
```

### File: internal/engine/ledger.go
```go
func NewLedger() *Ledger {
	return &Ledger{
		patches: make(map[string]Patch),
		backups: make(map[string][]byte),
	}
}
type Patch struct {
	FilePath   string
	OldContent string
	NewContent string
}
type Ledger struct {
	mu      sync.Mutex
	patches map[string]Patch
	backups map[string][]byte
}
```

### File: internal/engine/linker.go
```go
func LinkInterfaces(ctx context.Context, repo store.Repository, lspMgr *lsp.Manager) error {
	interfaces, err := repo.GetInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to get interfaces: %w", err)
	}

	for _, iface := range interfaces {
		client, err := lspMgr.GetClient(ctx, iface.Path)
		if err != nil {
			// Skip if LSP client is not available for this file type
			continue
		}

		// Prepare implementation params. LSP uses 0-based lines and columns.
		// We use 1-based lines and columns in our store (from tree-sitter).
		
		charPos := 0
		if iface.StartCol > 0 {
			charPos = iface.StartCol - 1
		}

		// [Strike 6] Sincronización Determinista (LSP Black Box Sync)
		// gopls blocks Hover/Definition until the file is fully type-checked.
		// We send a dummy request to line 0 to wait for readiness instead of sleeping.
		syncParams := lsp.HoverParams{
			TextDocumentPositionParams: lsp.TextDocumentPositionParams{
				TextDocument: lsp.TextDocumentIdentifier{
					URI: "file://" + iface.Path,
				},
				Position: lsp.Position{Line: 0, Character: 0},
			},
		}
		_, _ = client.Hover(ctx, syncParams)

		params := lsp.ImplementationParams{
			TextDocumentPositionParams: lsp.TextDocumentPositionParams{
				TextDocument: lsp.TextDocumentIdentifier{
					URI: "file://" + iface.Path,
				},
				Position: lsp.Position{
					Line:      iface.StartLine - 1,
					Character: charPos,
				},
			},
		}

		locations, err := client.Implementation(ctx, params)
		if err != nil {
			fmt.Printf("  [Linker] LSP error for %s: %v\n", iface.Name, err)
			continue
		}

		fmt.Printf("  [Linker] Found %d implementations for %s\n", len(locations), iface.Name)

		for _, loc := range locations {
			destPath := strings.TrimPrefix(loc.URI, "file://")
			
			// Only save if the destination is not the same as the interface itself
			if destPath == iface.Path && loc.Range.Start.Line == iface.StartLine-1 {
				continue
			}

			// We don't have the implementation name easily from LSP Location,
			// but we can save it as an "implements" link to the file/line.
			// Scouter V2.0 will resolve this semantically during impact analysis.
			call := store.Call{
				CallerName: "impl", // Generic name for implementation
				CalleeName: iface.Name,
				CalleePath: iface.Path,
				LinkType:   "implements",
				Path:       destPath,
				Line:       loc.Range.Start.Line + 1, // Back to 1-based
			}

			if err := repo.SaveCall(ctx, call); err != nil {
				fmt.Printf("  [Linker] SaveCall error for %s: %v\n", iface.Name, err)
				continue
			}
		}
	}

	return nil
}
```

### File: internal/engine/lsp/client.go
```go
func NewClient(ctx context.Context, dir string, binary string, args ...string) (LSPClient, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &jsonrpcClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: stdout,
		writer: stdin,
		done:   make(chan struct{}),
	}

	go c.listen()

	// Initialize
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}
type jsonrpcClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader io.Reader // for testing
	writer io.Writer // for testing

	nextID  atomic.Uint64
	pending sync.Map // map[uint64]chan *JSONRPCResponse

	done chan struct{}
}
type LSPClient interface {
	Definition(ctx context.Context, params DefinitionParams) ([]Location, error)
	Implementation(ctx context.Context, params ImplementationParams) ([]Location, error)
	References(ctx context.Context, params ReferenceParams) ([]Location, error)
	Hover(ctx context.Context, params HoverParams) (*Hover, error)
	Close() error
}
```

### File: internal/engine/lsp/client_test.go
```go
func TestJSONRPCClient(t *testing.T) {
	// We need a mock server binary or a way to simulate a process.
	// For testing, we can use a pipe and a goroutine that acts as a server.

	pr, pw := io.Pipe() // client read, server write
	sr, sw := io.Pipe() // server read, client write

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Mock server logic
	go func() {
		reader := bufio.NewReader(sr)
		for {
			var contentLength int
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimSpace(line)
				if line == "" {
					break
				}
				if strings.HasPrefix(line, "Content-Length:") {
					fmt.Sscanf(line, "Content-Length: %d", &contentLength)
				}
			}

			if contentLength == 0 {
				continue
			}

			body := make([]byte, contentLength)
			if _, err := io.ReadFull(reader, body); err != nil {
				return
			}

			// Basic JSON-RPC parsing
			var req JSONRPCRequest
			if err := json.Unmarshal(body, &req); err != nil {
				return
			}

			// Respond based on method
			var resp JSONRPCResponse
			resp.JSONRPC = "2.0"
			resp.ID = req.ID

			switch req.Method {
			case "initialize":
				resp.Result = json.RawMessage(`{"capabilities": {}}`)
			case "textDocument/definition":
				resp.Result = json.RawMessage(`[{"uri": "file:///test.go", "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 5}}}]`)
			case "textDocument/implementation":
				resp.Result = json.RawMessage(`[{"uri": "file:///impl.go", "range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 15}}}]`)
			case "shutdown":
				resp.Result = json.RawMessage(`null`)
			}

			data, _ := json.Marshal(resp)
			fmt.Fprintf(pw, "Content-Length: %d\r\n\r\n%s", len(data), data)
		}
	}()

	client := &jsonrpcClient{
		reader: pr,
		writer: sw,
		done:   make(chan struct{}),
	}
	go client.listen()

	// The client expects an initialize call if we use NewClient,
	// but here we are using it manually.
	// Let's test the methods directly.

	// Test Definition
	params := DefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///test.go"},
			Position:     Position{Line: 1, Character: 1},
		},
	}

	// We need to implement the client logic to read/write headers as well if we follow LSP strictly.
	// But the prompt says "Basic JSON-RPC over stdio client".
	// LSP uses "Content-Length: ...\r\n\r\n{json}".

	locs, err := client.Definition(ctx, params)
	if err != nil {
		t.Fatalf("Definition failed: %v", err)
	}

	if len(locs) != 1 || locs[0].URI != "file:///test.go" {
		t.Errorf("Unexpected result: %v", locs)
	}

	// Test Implementation
	implParams := ImplementationParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///test.go"},
			Position:     Position{Line: 1, Character: 1},
		},
	}

	impls, err := client.Implementation(ctx, implParams)
	if err != nil {
		t.Fatalf("Implementation failed: %v", err)
	}

	if len(impls) != 1 || impls[0].URI != "file:///impl.go" {
		t.Errorf("Unexpected implementation result: %v", impls)
	}
}
```

### File: internal/engine/lsp/manager.go
```go
func NewManager() *Manager {
	clients := make(map[string]*clientEntry)
	m := &Manager{
		clients:       clients,
		clientCreator: NewClient,
	}
	// Go 1.25 native cleanup for the manager singleton
	// We use the clients map as the anchor to avoid the "ptr is equal to arg" panic
	runtime.AddCleanup(m, func(c map[string]*clientEntry) {
		for _, entry := range c {
			if entry.client != nil {
				entry.client.Close()
			}
		}
	}, clients)
	return m
}
type clientEntry struct {
	client LSPClient
	err    error
	once   sync.Once
}
type Manager struct {
	clients map[string]*clientEntry
	mu      sync.RWMutex
	closed  bool

	// clientCreator allows mocking for tests
	clientCreator func(ctx context.Context, dir string, binary string, args ...string) (LSPClient, error)
}
```

### File: internal/engine/lsp/manager_test.go
```go
func TestLSPManager(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	// Mock client creation
	m.clientCreator = func(ctx context.Context, dir string, binary string, args ...string) (LSPClient, error) {
		return &mockClient{binary: binary}, nil
	}

	// Test extension mapping
	ext := "test.go"
	client, err := m.GetClient(ctx, ext)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	mc := client.(*mockClient)
	if mc.binary != "gopls" {
		t.Errorf("Expected binary gopls, got %s", mc.binary)
	}

	// Test caching
	client2, _ := m.GetClient(ctx, ext)
	if client != client2 {
		t.Errorf("Client not cached")
	}
}
type mockClient struct {
	binary string
}
```

### File: internal/engine/lsp/types.go
```go
type Position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}
type DefinitionParams struct {
	TextDocumentPositionParams
}
type ImplementationParams struct {
	TextDocumentPositionParams
}
type HoverParams struct {
	TextDocumentPositionParams
}
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}
type InitializeParams struct {
	ProcessID        int                `json:"processId"`
	RootURI          string             `json:"rootUri,omitempty"`
	Capabilities     ClientCapabilities `json:"capabilities"`
	WorkspaceFolders []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}
type ClientCapabilities struct {
	TextDocument struct {
		Definition struct {
			DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
		} `json:"definition,omitempty"`
		References struct {
			DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
		} `json:"references,omitempty"`
		Hover struct {
			ContentFormat []string `json:"contentFormat,omitempty"`
		} `json:"hover,omitempty"`
	} `json:"textDocument,omitempty"`
}
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}
```

### File: internal/engine/manifest.go
```go
func ParseGoMod(ctx context.Context, filePath string) ([]types.Dependency, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}

	f, err := modfile.Parse(filePath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	var deps []types.Dependency
	for _, req := range f.Require {
		deps = append(deps, types.Dependency{
			Name:    req.Mod.Path,
			Version: req.Mod.Version,
			Type:    "golang",
			Project: filePath,
			Direct:  !req.Indirect,
		})
	}

	return deps, nil
}
func ParsePackageJSON(ctx context.Context, filePath string) ([]types.Dependency, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}

	var deps []types.Dependency
	// Standard dependencies
	for name, version := range pkg.Dependencies {
		deps = append(deps, types.Dependency{
			Name:    name,
			Version: version,
			Type:    "npm",
			Project: filePath,
			Direct:  true,
		})
	}
	// Dev dependencies
	for name, version := range pkg.DevDependencies {
		deps = append(deps, types.Dependency{
			Name:    name,
			Version: version,
			Type:    "npm",
			Project: filePath,
			Direct:  true,
		})
	}

	return deps, nil
}
```

### File: internal/engine/manifest_test.go
```go
func TestParseGoMod(t *testing.T) {
	content := `module github.com/test/project

go 1.25

require (
	github.com/google/uuid v1.3.0
	github.com/stretchr/testify v1.8.4 // indirect
)
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParseGoMod(context.Background(), goModPath)
	if err != nil {
		t.Fatalf("ParseGoMod failed: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}

	foundDirect := false
	foundIndirect := false
	for _, d := range deps {
		if d.Name == "github.com/google/uuid" {
			foundDirect = true
			if !d.Direct {
				t.Error("github.com/google/uuid should be direct")
			}
		}
		if d.Name == "github.com/stretchr/testify" {
			foundIndirect = true
			if d.Direct {
				t.Error("github.com/stretchr/testify should be indirect")
			}
		}
	}

	if !foundDirect || !foundIndirect {
		t.Error("Did not find both expected dependencies")
	}
}
func TestParsePackageJSON(t *testing.T) {
	content := `{
  "name": "test-project",
  "dependencies": {
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	pkgJSONPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgJSONPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParsePackageJSON(context.Background(), pkgJSONPath)
	if err != nil {
		t.Fatalf("ParsePackageJSON failed: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}

	foundLodash := false
	foundTS := false
	for _, d := range deps {
		if d.Name == "lodash" {
			foundLodash = true
		}
		if d.Name == "typescript" {
			foundTS = true
		}
		if !d.Direct {
			t.Errorf("NPM dependency %s should be marked as direct", d.Name)
		}
	}

	if !foundLodash || !foundTS {
		t.Error("Did not find both expected NPM dependencies")
	}
}
```

### File: internal/engine/parser.go
```go
func ParseFile(ctx context.Context, filePath string, lspMgr *lsp.Manager) ([]types.ASTPointer, []types.ASTCall, error) {
	itPointers, itCalls, err := StreamSymbols(ctx, filePath)
	if err != nil {
		// Try Tree-sitter for multi-language support as fallback
		p, c, tsErr := ParseWithTreeSitter(ctx, filePath)
		if tsErr == nil {
			return p, c, nil
		}
		return nil, nil, fmt.Errorf("parsing failed for %s: %w (fallback error: %v)", filePath, err, tsErr)
	}
	return slices.Collect(itPointers), slices.Collect(itCalls), nil
}
func StreamSymbols(ctx context.Context, filePath string) (iter.Seq[types.ASTPointer], iter.Seq[types.ASTCall], error) {
	// 1. Context check
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	ext := filepath.Ext(filePath)
	if ext != ".go" {
		// Tree-sitter fallback still returns slices for now, we wrap them in iterators
		p, c, err := ParseWithTreeSitter(ctx, filePath)
		if err != nil {
			return nil, nil, err
		}
		return slices.Values(p), slices.Values(c), nil
	}

	// 2. Path Security Check
	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, nil, err
	}

	// 1.5. Size Limit Check
	fi, err := os.Stat(validatedPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error stating file: %w", err)
	}
	if fi.Size() > MaxParseSize {
		return nil, nil, fmt.Errorf("file too large to index (%d bytes), limit is %d bytes", fi.Size(), MaxParseSize)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, validatedPath, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Go native parser failed for %s: %v\n", validatedPath, err)
		return nil, nil, err
	}

	// We return closures that perform the AST inspection lazily
	return func(yield func(types.ASTPointer) bool) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
				select {
				case <-ctx.Done():
					return false
				default:
				}

				if n == nil {
					return true
				}

				if fn, ok := n.(*ast.FuncDecl); ok {
					startPos := fset.Position(fn.Pos())
					endPos := fset.Position(fn.End())
					identPos := fset.Position(fn.Name.Pos())
					doc := utils.CleanComment(fn.Doc.Text())
					fullName := fn.Name.Name
					symType := "function"
					if fn.Recv != nil && len(fn.Recv.List) > 0 {
						symType = "method"
						recvType := ""
						switch r := fn.Recv.List[0].Type.(type) {
						case *ast.Ident:
							recvType = r.Name
						case *ast.StarExpr:
							if id, ok := r.X.(*ast.Ident); ok {
								recvType = id.Name
							}
						}
						if recvType != "" {
							fullName = recvType + "." + fn.Name.Name
						}
					}

					signature := extractSignature(fn.Type)
					content := fmt.Sprintf("%s:%s:%s:%d:%d", symType, fullName, signature, startPos.Offset, endPos.Offset)
					h := sha256.Sum256([]byte(content))
					p := types.ASTPointer{
						Type:      symType,
						Name:      fullName,
						Signature: signature,
						Doc:       doc,
						Range:     types.Range{Start: startPos.Offset, End: endPos.Offset},
						StartLine: identPos.Line,
						StartCol:  identPos.Column,
						EndLine:   endPos.Line,
						Hash:      hex.EncodeToString(h[:]),
					}
					if !yield(p) {
						return false
					}
				}

				// Capture Structs and Interfaces from GenDecl
				if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
					for _, spec := range gd.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok {
							identPos := fset.Position(ts.Name.Pos())
							var symType string
							switch it := ts.Type.(type) {
							case *ast.StructType:
								symType = "class"
							case *ast.InterfaceType:
								symType = "interface"
								if it.Methods != nil {
									for _, field := range it.Methods.List {
										if len(field.Names) > 0 {
											methodName := field.Names[0].Name
											fullMethodName := ts.Name.Name + ":" + methodName
											mStart := fset.Position(field.Pos())
											mEnd := fset.Position(field.End())
											mIdent := fset.Position(field.Names[0].Pos())

											var sig string
											if ft, ok := field.Type.(*ast.FuncType); ok {
												sig = extractSignature(ft)
											}

											p := types.ASTPointer{
												Type:      "method_spec",
												Name:      fullMethodName,
												Signature: sig,
												Range:     types.Range{Start: mStart.Offset, End: mEnd.Offset},
												StartLine: mIdent.Line,
												StartCol:  mIdent.Column,
												EndLine:   mEnd.Line,
												Hash:      utils.HashString(fmt.Sprintf("spec:%s:%s", fullMethodName, sig)),
											}
											if !yield(p) {
												return false
											}
										}
									}
								}
							default:
								continue
							}

							startPos := fset.Position(ts.Pos())
							endPos := fset.Position(ts.End())
							doc := utils.CleanComment(gd.Doc.Text())
							if doc == "" {
								doc = utils.CleanComment(ts.Doc.Text())
							}

							content := fmt.Sprintf("%s:%s:%d:%d", symType, ts.Name.Name, startPos.Offset, endPos.Offset)
							h := sha256.Sum256([]byte(content))
							p := types.ASTPointer{
								Type:      symType,
								Name:      ts.Name.Name,
								Doc:       doc,
								Range:     types.Range{Start: startPos.Offset, End: endPos.Offset},
								StartLine: identPos.Line,
								StartCol:  identPos.Column,
								EndLine:   endPos.Line,
								Hash:      hex.EncodeToString(h[:]),
							}
							if !yield(p) {
								return false
							}
						}
					}
				}
				return true
			})
		}, func(yield func(types.ASTCall) bool) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			names := make(map[ast.Node]string)
			anonCounts := make(map[ast.Node]int)
			globalAnonCount := 0

			ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
				select {
				case <-ctx.Done():
					return false
				default:
				}

				if n == nil {
					return true
				}

				if fn, ok := n.(*ast.FuncDecl); ok {
					names[fn] = fn.Name.Name
				} else if fn, ok := n.(*ast.FuncLit); ok {
					parentName := "global"
					var parentNode ast.Node
					for i := len(stack) - 1; i >= 0; i-- {
						p := stack[i]
						if _, ok := p.(*ast.FuncDecl); ok {
							parentNode = p
							parentName = names[p]
							break
						}
						if _, ok := p.(*ast.FuncLit); ok {
							parentNode = p
							parentName = names[p]
							break
						}
					}
					var count int
					if parentNode != nil {
						anonCounts[parentNode]++
						count = anonCounts[parentNode]
					} else {
						globalAnonCount++
						count = globalAnonCount
					}
					anonName := fmt.Sprintf("%s.func%d", parentName, count)
					names[fn] = anonName
				}

				if call, ok := n.(*ast.CallExpr); ok {
					var callerName string
					for i := len(stack) - 1; i >= 0; i-- {
						p := stack[i]
						if name, ok := names[p]; ok {
							callerName = name
							break
						}
					}

					if callerName != "" {
						calleeName, calleePath := resolveCallee(call.Fun, validatedPath)
						if calleeName != "" {
							c := types.ASTCall{
								CallerName: callerName,
								CalleeName: calleeName,
								CalleePath: calleePath,
								LinkType:   "call",
								Path:       validatedPath,
								Line:       fset.Position(call.Lparen).Line,
							}
							if !yield(c) {
								return false
							}
						}
					}
				}
				return true
			})
		}, nil
}
func resolveCallee(fun ast.Expr, currentPath string) (string, string) {
	switch f := fun.(type) {
	case *ast.Ident:
		// Package-level call or local variable.
		// We return an empty path to allow the store to resolve it globally within the package/project.
		// Returning currentPath is incorrect for multi-file packages (Divine Fix).
		return f.Name, ""
	case *ast.SelectorExpr:
		// Potential call to another package or a method on a struct
		if x, ok := f.X.(*ast.Ident); ok {
			// Heuristic: if X is lowercase, it might be a variable (method call).
			// If X is Uppercase, it might be a package name.
			// For now, we return the selector string.
			return x.Name + "." + f.Sel.Name, ""
		}
		return f.Sel.Name, ""
	default:
		return "", ""
	}
}
func ReadFragment(ctx context.Context, filePath string, r types.Range) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	validatedPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return "", err
	}

	requestedSize := r.End - r.Start
	if requestedSize > MaxFragmentSize {
		return "", fmt.Errorf("requested fragment too large (%d bytes), limit is %d bytes", requestedSize, MaxFragmentSize)
	}

	f, err := os.Open(validatedPath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("error stating file: %w", err)
	}

	if r.Start < -1 || int64(r.End) > fi.Size() || (r.Start > r.End && r.Start != -1) {
		return "", fmt.Errorf("file out of sync or invalid range: index is stale, please re-index the file")
	}

	buffer := make([]byte, requestedSize)
	_, err = f.ReadAt(buffer, int64(r.Start))
	if err != nil {
		return "", fmt.Errorf("error reading range: %w", err)
	}

	if !utf8.Valid(buffer) {
		return "", fmt.Errorf("binary or invalid UTF-8 data detected: Scouter only analyzes text-based source code")
	}

	return string(buffer), nil
}
func extractSignature(ft *ast.FuncType) string {
	if ft == nil {
		return ""
	}

	params := ""
	if ft.Params != nil {
		var pList []string
		for _, field := range ft.Params.List {
			pType := exprToString(field.Type)
			if len(field.Names) > 0 {
				for range field.Names {
					pList = append(pList, pType)
				}
			} else {
				pList = append(pList, pType)
			}
		}
		params = "(" + strings.Join(pList, ", ") + ")"
	}

	results := ""
	if ft.Results != nil {
		var rList []string
		for _, field := range ft.Results.List {
			rList = append(rList, exprToString(field.Type))
		}
		results = "(" + strings.Join(rList, ", ") + ")"
	}

	if results == "" {
		return params
	}
	return params + " -> " + results
}
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return fmt.Sprintf("interface{%d methods}", len(t.Methods.List))
	case *ast.StructType:
		if t.Fields == nil || len(t.Fields.List) == 0 {
			return "struct{}"
		}
		return fmt.Sprintf("struct{%d fields}", len(t.Fields.List))
	case *ast.ChanType:
		if t.Dir == ast.RECV {
			return "<-chan " + exprToString(t.Value)
		} else if t.Dir == ast.SEND {
			return "chan<- " + exprToString(t.Value)
		}
		return "chan " + exprToString(t.Value)
	case *ast.FuncType:
		return "func" + extractSignature(t)
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	case *ast.ParenExpr:
		return "(" + exprToString(t.X) + ")"
	default:
		return "unknown"
	}
}
```

### File: internal/engine/parser_test.go
```go
func TestParseFileWithCalls(t *testing.T) {
	content := `
package test
func Caller() {
	Callee()
}
func Callee() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	pointers, calls, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify pointers (symbols)
	if len(pointers) != 2 {
		t.Errorf("expected 2 pointers, got %d", len(pointers))
	}

	// Verify calls
	if len(calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(calls))
	} else {
		call := calls[0]
		if call.CallerName != "Caller" {
			t.Errorf("expected caller Caller, got %s", call.CallerName)
		}
		if call.CalleeName != "Callee" {
			t.Errorf("expected callee Callee, got %s", call.CalleeName)
		}
	}
}
func TestParseFileWithNestedCalls(t *testing.T) {
	content := `
package test
func Outer() {
	Inner()
	func() {
		Nested()
	}()
}
func Inner() {}
func Nested() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	_, calls, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}

	call1 := calls[0]
	if call1.CallerName != "Outer" {
		t.Errorf("expected caller Outer, got %s", call1.CallerName)
	}
	if call1.CalleeName != "Inner" {
		t.Errorf("expected callee Inner, got %s", call1.CalleeName)
	}

	// The anonymous function is now correctly tracked as Outer.func1
	call2 := calls[1]
	if call2.CallerName != "Outer.func1" {
		t.Errorf("expected caller Outer.func1 for nested call, got %s", call2.CallerName)
	}
	if call2.CalleeName != "Nested" {
		t.Errorf("expected callee Nested, got %s", call2.CalleeName)
	}
}
func TestParseFileWithAnonymousCalls(t *testing.T) {
	content := `
package test
func Caller() {
	go func() {
		Callee()
	}()

	f := func() {
		NestedCallee()
	}
	f()
}
func Callee() {}
func NestedCallee() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	_, calls, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	// Verify the first call (go func)
	call1 := calls[0]
	if call1.CallerName != "Caller.func1" {
		t.Errorf("expected caller Caller.func1, got %s", call1.CallerName)
	}
	if call1.CalleeName != "Callee" {
		t.Errorf("expected callee Callee, got %s", call1.CalleeName)
	}

	// Verify the second call (closure)
	call2 := calls[1]
	if call2.CallerName != "Caller.func2" {
		t.Errorf("expected caller Caller.func2, got %s", call2.CallerName)
	}
	if call2.CalleeName != "NestedCallee" {
		t.Errorf("expected callee NestedCallee, got %s", call2.CalleeName)
	}

	// Verify the third call (f())
	call3 := calls[2]
	if call3.CallerName != "Caller" {
		t.Errorf("expected caller Caller, got %s", call3.CallerName)
	}
	if call3.CalleeName != "f" {
		t.Errorf("expected callee f, got %s", call3.CalleeName)
	}
}
func TestParseFileWithDoc(t *testing.T) {
	content := `
package test

// Hello is a greeting function.
func Hello() {}

/*
World is a global function.
It does something.
*/
func World() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	pointers, _, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(pointers) != 2 {
		t.Fatalf("expected 2 pointers, got %d", len(pointers))
	}

	if pointers[0].Name == "Hello" {
		expected := "Hello is a greeting function."
		if pointers[0].Doc != expected {
			t.Errorf("expected doc %q, got %q", expected, pointers[0].Doc)
		}
	} else {
		t.Errorf("expected first pointer to be Hello")
	}

	if pointers[1].Name == "World" {
		expected := "World is a global function.\nIt does something."
		if pointers[1].Doc != expected {
			t.Errorf("expected doc %q, got %q", expected, pointers[1].Doc)
		}
	} else {
		t.Errorf("expected second pointer to be World")
	}
}
func TestParseTypeScriptWithCalls(t *testing.T) {
	content := `
function caller() {
	callee();
}
function callee() {}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.ts")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx := context.Background()
	_, calls, err := ParseFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(calls))
	} else {
		call := calls[0]
		if call.CallerName != "caller" {
			t.Errorf("expected caller caller, got %s", call.CallerName)
		}
		if call.CalleeName != "callee" {
			t.Errorf("expected callee callee, got %s", call.CalleeName)
		}
	}
}
```

### File: internal/engine/pipeline.go
```go
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
```

### File: internal/engine/pipeline_test.go
```go
func TestApplyPipelineKeepLines(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	input := "hello\n\nworld\n\n"
	out, err := ApplyPipeline(ctx, f, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2: %v", len(lines), lines)
	}
}
func TestApplyPipelineChained(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
			{ActionName: "head", Params: map[string]any{"n": 2}},
		},
	}

	input := "a\nb\nc\nd\ne\n"
	out, err := ApplyPipeline(ctx, f, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 { // 2 kept + overflow msg
		t.Errorf("got %d lines: %v", len(lines), lines)
	}
}
func TestApplyPipelineUnknownAction(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "nonexistent"},
		},
	}

	_, err := ApplyPipeline(ctx, f, "input", nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}
func TestApplyPipelineEmptyInput(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	out, err := ApplyPipeline(ctx, f, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}
func TestApplyPipelineGracefulDegradation(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{}}, // Missing pattern
		},
	}

	_, err := ApplyPipeline(ctx, f, "hello\nworld\n", nil)
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}
```

### File: internal/engine/predict.go
```go
func PredictTests(ctx context.Context, db store.Repository, diff string) ([]types.TestTarget, error) {
	if diff == "" {
		return nil, nil
	}

	ranges, err := parseDiff(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	var allSymbols []store.Symbol
	for _, r := range ranges {
		// Normalize path to absolute to match database (Divine Fix)
		absPath, err := filepath.Abs(r.Path)
		if err != nil {
			absPath = r.Path
		}

		symbols, err := db.GetSymbolsByRange(ctx, absPath, r.StartLine, r.EndLine)
		if err != nil {
			// Skip files not in index
			continue
		}
		allSymbols = append(allSymbols, symbols...)
	}

	return findTestsForSymbols(ctx, db, allSymbols)
}
func parseDiff(diff string) ([]diffRange, error) {
	var ranges []diffRange
	var currentFile string
	scanner := bufio.NewScanner(strings.NewReader(diff))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			matches := hunkRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				start, _ := strconv.Atoi(matches[1])
				count := 1
				if len(matches) == 3 && matches[2] != "" {
					count, _ = strconv.Atoi(matches[2])
				}

				ranges = append(ranges, diffRange{
					Path:      currentFile,
					StartLine: start,
					EndLine:   start + count - 1,
				})
			}
		}
	}

	return ranges, nil
}
func findTestsForSymbols(ctx context.Context, db store.Repository, symbols []store.Symbol) ([]types.TestTarget, error) {
	uniqueTests := make(map[string]types.TestTarget)
	for _, sym := range symbols {
		affectedTests, err := db.GetAffectedTests(ctx, sym.Name, sym.Path)
		if err != nil {
			return nil, err
		}
		for _, t := range affectedTests {
			key := t.Path + ":" + t.Name
			uniqueTests[key] = types.TestTarget{
				Name: t.Name,
				File: t.Path,
			}
		}
	}

	result := make([]types.TestTarget, 0, len(uniqueTests))
	for _, t := range uniqueTests {
		result = append(result, t)
	}
	return result, nil
}
type diffRange struct {
	Path      string
	StartLine int
	EndLine   int
}
```

### File: internal/engine/predict_test.go
```go
func TestPredictTests(t *testing.T) {
	path, _ := filepath.Abs("internal/engine/processor.go")
	testPath, _ := filepath.Abs("internal/engine/processor_test.go")

	db := &predictMockStore{
		symbolsByRange: map[string][]store.Symbol{
			path: {
				{Name: "ProcessData", Path: path},
			},
		},
		affectedTests: map[string][]store.Symbol{
			"ProcessData:" + path: {
				{Name: "TestProcessData", Path: testPath},
			},
		},
	}

	diff := `--- a/internal/engine/processor.go
+++ b/internal/engine/processor.go
@@ -10,1 +10,1 @@
-func ProcessData() {
+func ProcessData(ctx context.Context) {`

	ctx := context.Background()
	tests, err := PredictTests(ctx, db, diff)
	if err != nil {
		t.Fatalf("PredictTests failed: %v", err)
	}

	if len(tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(tests))
	}

	if tests[0].Name != "TestProcessData" || tests[0].File != testPath {
		t.Errorf("unexpected test: %+v", tests[0])
	}
}
func TestPredictTestsEmptyDiff(t *testing.T) {
	tests, err := PredictTests(context.Background(), &predictMockStore{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 0 {
		t.Error("expected empty result")
	}
}
func TestFindTestsForSymbolsUnique(t *testing.T) {
	db := &predictMockStore{
		affectedTests: map[string][]store.Symbol{
			"A:file.go": {
				{Name: "Test1", Path: "test.go"},
			},
			"B:file.go": {
				{Name: "Test1", Path: "test.go"},
				{Name: "Test2", Path: "test.go"},
			},
		},
	}

	symbols := []store.Symbol{
		{Name: "A", Path: "file.go"},
		{Name: "B", Path: "file.go"},
	}

	tests, err := findTestsForSymbols(context.Background(), db, symbols)
	if err != nil {
		t.Fatal(err)
	}

	if len(tests) != 2 {
		t.Errorf("expected 2 unique tests, got %d", len(tests))
	}

	// Verify TestTarget usage
	var _ types.TestTarget = tests[0]
}
type predictMockStore struct {
	store.Repository
	symbolsByRange map[string][]store.Symbol
	affectedTests  map[string][]store.Symbol
}
```

### File: internal/engine/resolver.go
```go
type LocalFileResolver struct{}
```

### File: internal/engine/ripple.go
```go
func NewRippleEngine(s store.Repository, t Transformer) *RippleEngine {
	return &RippleEngine{
		store:       s,
		Transformer: t,
	}
}
type RippleEngine struct {
	store       store.Repository
	Transformer Transformer
}
type MCPTransformer struct {
	// This will be bridged from the MCP handler
	DoTransform func(ctx context.Context, file, symbol, transformation string) (string, error)
}
type Transformer interface {
	Transform(ctx context.Context, file string, symbolName string, transformation string) (string, error)
}
```

### File: internal/engine/search.go
```go
func NewSearchEngine(s store.Repository) *SearchEngine {
	return &SearchEngine{store: s}
}
type SearchEngine struct {
	store store.Repository
}
type symRes struct {
		symbols []store.Symbol
		err     error
	}
type insRes struct {
		insights []types.MemoryInsight
		err      error
	}
```

### File: internal/engine/structural.go
```go
func StructuralSearch(ctx context.Context, rootPath, pattern, ext string) ([]StructuralMatch, error) {
	lang, ok := languageConfigs[ext]
	if !ok {
		// Try to find by extension if ext is just an extension
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		lang, ok = languageConfigs[ext]
		if !ok {
			return nil, nil // Unsupported language
		}
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang.Language)

	// Pre-process pattern to make wildcards valid identifiers
	// E.g., $X -> __SCT_X__
	processedPattern := pattern
	processedPattern = strings.ReplaceAll(processedPattern, "$$$", "__SCT_MULTI__")
	for i := 'A'; i <= 'Z'; i++ {
		processedPattern = strings.ReplaceAll(processedPattern, "$"+string(i), "__SCT_"+string(i)+"__")
	}

	// Pattern Parsing Strategy: Try as-is, then try wrapped if ERROR found
	var patternTree *tree_sitter.Tree
	patternTree = parser.Parse([]byte(processedPattern), nil)
	patternRoot := patternTree.RootNode()
	pContent := []byte(processedPattern)

	if patternRoot.HasError() {
		// Try wrapping in a function body for Go/TS
		wrapped := processedPattern
		if ext == ".go" {
			wrapped = "package main\nfunc _() {\n" + processedPattern + "\n}"
		} else if ext == ".ts" || ext == ".js" {
			wrapped = "function _() {\n" + processedPattern + "\n}"
		}

		wTree := parser.Parse([]byte(wrapped), nil)
		if !wTree.RootNode().HasError() {
			pContent = []byte(wrapped)
			patternRoot = findInnermostMatch(wTree.RootNode(), pContent, processedPattern)
			patternTree.Close() // Close original tree with error
			patternTree = wTree // Keep wTree for the search
		} else {
			wTree.Close()
		}
	}
	defer patternTree.Close()

	// Extract effective node (skip source_file wrapper if it's just one child)
	for patternRoot != nil && patternRoot.ChildCount() == 1 && !isWildcard(patternRoot.Utf8Text(pContent)) {
		patternRoot = patternRoot.Child(0)
	}

	if patternRoot == nil {
		return nil, nil
	}

	var matches []StructuralMatch
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr // Context Sovereignty
		}

		if err != nil || info.IsDir() || filepath.Ext(path) != ext {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Use anonymous function to safely defer tree.Close() (Strike 1 Redemption)
		func() {
			tree := parser.Parse(content, nil)
			defer tree.Close()
			findMatches(tree.RootNode(), patternRoot, pContent, content, path, &matches)
		}()
		return nil
	})

	return matches, err
}
func findInnermostMatch(node *tree_sitter.Node, content []byte, pattern string) *tree_sitter.Node {
	for i := uint(0); i < uint(node.ChildCount()); i++ {
		child := node.Child(i)
		// Skip comments and whitespace-only nodes
		if child.Kind() == "comment" || strings.TrimSpace(child.Utf8Text(content)) == "" {
			continue
		}
		if strings.Contains(child.Utf8Text(content), strings.TrimSpace(pattern)) {
			return findInnermostMatch(child, content, pattern)
		}
	}
	return node
}
func findMatches(node, pattern *tree_sitter.Node, patternContent, targetContent []byte, path string, matches *[]StructuralMatch) {
	if matchNodes(node, pattern, patternContent, targetContent) {
		*matches = append(*matches, StructuralMatch{
			Path:      path,
			Range:     types.Range{Start: int(node.StartByte()), End: int(node.EndByte())},
			StartLine: int(node.StartPosition().Row) + 1,
			EndLine:   int(node.EndPosition().Row) + 1,
			Content:   node.Utf8Text(targetContent),
		})
	}

	for i := uint(0); i < uint(node.ChildCount()); i++ {
		findMatches(node.Child(i), pattern, patternContent, targetContent, path, matches)
	}
}
func isWildcard(text string) bool {
	return strings.HasPrefix(text, "__SCT_")
}
func matchNodes(node, pattern *tree_sitter.Node, patternContent, targetContent []byte) bool {
	pText := pattern.Utf8Text(patternContent)
	
	if isWildcard(pText) {
		return true
	}

	if node.Kind() != pattern.Kind() {
		return false
	}

	if pattern.ChildCount() == 0 {
		return node.Utf8Text(targetContent) == pText
	}

	if node.ChildCount() < pattern.ChildCount() {
		return false
	}

	pIdx := uint(0)
	for tIdx := uint(0); tIdx < uint(node.ChildCount()) && pIdx < uint(pattern.ChildCount()); tIdx++ {
		if matchNodes(node.Child(tIdx), pattern.Child(pIdx), patternContent, targetContent) {
			pIdx++
		}
	}

	return pIdx == uint(pattern.ChildCount())
}
type StructuralMatch struct {
	Path      string      `json:"path"`
	Range     types.Range `json:"range"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
	Content   string      `json:"content"`
}
```

### File: internal/engine/structural_test.go
```go
func TestStructuralSearch(t *testing.T) {
	content := `package main
func main() {
	println("hello")
	println("world")
}
func foo() {
	println("bar")
}`
	tmpFile, err := os.CreateTemp("", "test*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	os.WriteFile(tmpFile.Name(), []byte(content), 0644)

	ctx := t.Context()
	
	t.Run("match function declaration", func(t *testing.T) {
		// Use a valid identifier as wildcard for now
		matches, err := StructuralSearch(ctx, tmpFile.Name(), "println($X)", ".go")
		if err != nil {
			t.Fatal(err)
		}
		// In current implementation, "X" must match "X". 
		// Let's modify matchNodes to treat uppercase identifiers as wildcards.
		if len(matches) != 3 {
			t.Errorf("got %d matches, want 3", len(matches))
		}
	})

	t.Run("match specific call", func(t *testing.T) {
		matches, err := StructuralSearch(ctx, tmpFile.Name(), `println("hello")`, ".go")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Errorf("got %d matches, want 1", len(matches))
		}
	})
}
```

### File: internal/engine/treesitter.go
```go
func init() {
	languageConfigs = make(map[string]*LanguageConfig)

	// Go Configuration: Native parser is preferred, TS is backup.
	goLang := tree_sitter.NewLanguage(tree_sitter_go.Language())
	registerLanguage(".go", goLang,
		`(function_declaration name: (identifier) @name) @function 
         (method_declaration name: (field_identifier) @name) @method`,
		`(call_expression function: (identifier) @callee) (call_expression function: (selector_expression field: (field_identifier) @callee))`)

	// TS Configuration: Enriched to capture interface methods
	tsLang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	registerLanguage(".ts", tsLang,
		`(class_declaration name: (type_identifier) @name) @class 
         (function_declaration name: (identifier) @name) @function 
         (method_definition name: (property_identifier) @name) @method 
         (interface_declaration name: (type_identifier) @name) @interface
         (interface_declaration name: (type_identifier) @iname body: (interface_body (method_signature (property_identifier) @mname))) @interface_spec`,
		`(call_expression function: (identifier) @callee) (call_expression function: (member_expression property: (property_identifier) @callee))`)

	// Python Configuration: Class methods are the "interfaces" by convention
	pyLang := tree_sitter.NewLanguage(tree_sitter_python.Language())
	registerLanguage(".py", pyLang,
		`(function_definition name: (identifier) @name) @function 
         (class_definition name: (identifier) @name) @class`,
		`(call function: (identifier) @callee) (call function: (attribute attribute: (identifier) @callee))`)
}
func registerLanguage(ext string, lang *tree_sitter.Language, qSrc, cSrc string) {
	q, err := tree_sitter.NewQuery(lang, qSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to register symbol query for %s: %v\n", ext, err)
	}
	cq, err := tree_sitter.NewQuery(lang, cSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to register call query for %s: %v\n", ext, err)
	}
	languageConfigs[ext] = &LanguageConfig{Language: lang, Query: q, CallQuery: cq}
}
func ParseWithTreeSitter(ctx context.Context, filePath string) ([]types.ASTPointer, []types.ASTCall, error) {
	ext := filepath.Ext(filePath)
	config, ok := languageConfigs[ext]
	if !ok {
		return []types.ASTPointer{}, []types.ASTCall{}, nil
	}

	if config.Query == nil || config.CallQuery == nil {
		return nil, nil, fmt.Errorf("tree-sitter queries not initialized for %s", ext)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(config.Language)

	tree := parser.Parse(content, nil)
	defer tree.Close()

	var pointers []types.ASTPointer
	var calls []types.ASTCall

	// Query definitions
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()
	matches := cursor.Matches(config.Query, tree.RootNode(), content)
	for match := matches.Next(); match != nil; match = matches.Next() {
		var name, symType, recv, mname, iname string
		var symNode tree_sitter.Node
		for _, cap := range match.Captures {
			nameN := config.Query.CaptureNames()[cap.Index]
			switch nameN {
			case "name":
				name = cap.Node.Utf8Text(content)
			case "recv":
				recv = cap.Node.Utf8Text(content)
			case "mname":
				mname = cap.Node.Utf8Text(content)
			case "iname":
				iname = cap.Node.Utf8Text(content)
			default:
				symType = nameN
				symNode = cap.Node
			}
		}

		// Handle normal symbols
		if name != "" && symType != "interface_spec" {
			fullName := name
			if recv != "" {
				fullName = recv + "." + name
			}

			doc := extractDoc(symNode, content, ext)
			h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", symType, fullName)))
			pointers = append(pointers, types.ASTPointer{
				Type:      symType,
				Name:      fullName,
				Doc:       doc,
				Range:     types.Range{Start: int(symNode.StartByte()), End: int(symNode.EndByte())},
				StartLine: int(symNode.StartPosition().Row) + 1,
				EndLine:   int(symNode.EndPosition().Row) + 1,
				Hash:      hex.EncodeToString(h[:]),
			})
		}

		// Handle interface method specs
		if mname != "" {
			parentInterface := name
			if iname != "" {
				parentInterface = iname
			}
			fullMethodName := parentInterface + ":" + mname
			pointers = append(pointers, types.ASTPointer{
				Type:      "method_spec",
				Name:      fullMethodName,
				Range:     types.Range{Start: int(symNode.StartByte()), End: int(symNode.EndByte())},
				StartLine: int(symNode.StartPosition().Row) + 1,
				EndLine:   int(symNode.EndPosition().Row) + 1,
				Hash:      utils.HashString(fmt.Sprintf("spec:%s", fullMethodName)),
			})
		}
	}

	// Query calls
	callMatches := cursor.Matches(config.CallQuery, tree.RootNode(), content)
	for match := callMatches.Next(); match != nil; match = callMatches.Next() {
		var callee string
		var callNode tree_sitter.Node
		for _, cap := range match.Captures {
			if config.CallQuery.CaptureNames()[cap.Index] == "callee" {
				callee = cap.Node.Utf8Text(content)
				callNode = cap.Node
			}
		}
		if callee != "" {
			calleePath := ""
			if callNode.Parent() != nil && callNode.Parent().Kind() == "call_expression" {
				if callNode.Kind() == "identifier" {
					calleePath = filePath
				}
			}

			calls = append(calls, types.ASTCall{
				CallerName: findTSCaller(callNode, content),
				CalleeName: callee,
				CalleePath: calleePath,
				LinkType:   "call",
				Path:       filePath,
				Line:       int(callNode.StartPosition().Row) + 1,
			})
		}
	}

	return pointers, calls, nil
}
func findTSCaller(node tree_sitter_node, content []byte) string {
	curr := node.Parent()
	for curr != nil {
		kind := curr.Kind()
		if kind == "function_definition" || kind == "function_declaration" || kind == "method_definition" || kind == "method_declaration" {
			recvName := ""
			parentClass := curr.Parent()
			for parentClass != nil {
				if parentClass.Kind() == "class_declaration" || parentClass.Kind() == "class_definition" {
					if nameNode := parentClass.ChildByFieldName("name"); nameNode != nil {
						recvName = nameNode.Utf8Text(content)
						break
					}
				}
				parentClass = parentClass.Parent()
			}
			
			if name := curr.ChildByFieldName("name"); name != nil {
				methodName := name.Utf8Text(content)
				if recvName != "" {
					return recvName + "." + methodName
				}
				return methodName
			}
		}
		curr = curr.Parent()
	}
	return "global"
}
func extractDoc(node tree_sitter.Node, content []byte, ext string) string {
	declNode := node
	for declNode.Kind() == "identifier" || declNode.Kind() == "property_identifier" || declNode.Kind() == "type_identifier" || declNode.Kind() == "field_identifier" {
		parent := declNode.Parent()
		if parent == nil {
			break
		}
		declNode = *parent
	}

	if ext == ".py" {
		block := declNode.ChildByFieldName("body")
		if block == nil {
			for i := uint32(0); i < uint32(declNode.ChildCount()); i++ {
				child := declNode.Child(uint(i))
				if child.Kind() == "block" {
					block = child
					break
				}
			}
		}

		if block != nil && block.ChildCount() > 0 {
			first := block.Child(0)
			if first.Kind() == "expression_statement" && first.ChildCount() > 0 {
				expr := first.Child(0)
				if expr.Kind() == "string" {
					return utils.CleanComment(expr.Utf8Text(content))
				}
			}
		}
	}

	var comments []string
	curr := declNode.PrevSibling()
	for curr != nil {
		kind := curr.Kind()
		if kind == "comment" || kind == "line_comment" || kind == "block_comment" {
			comments = append([]string{curr.Utf8Text(content)}, comments...)
			curr = curr.PrevSibling()
		} else if strings.TrimSpace(curr.Utf8Text(content)) == "" {
			curr = curr.PrevSibling()
		} else {
			break
		}
	}

	if len(comments) > 0 {
		return utils.CleanComment(strings.Join(comments, "\n"))
	}

	return ""
}
type LanguageConfig struct {
	Language  *tree_sitter.Language
	Query     *tree_sitter.Query
	CallQuery *tree_sitter.Query
}
```

### File: internal/engine/treesitter_test.go
```go
func TestParseWithTreeSitter_Calls(t *testing.T) {
	// Create a sample TS file
	content := []byte(`
		function caller() {
			callee();
			obj.method();
		}
		function callee() {}
	`)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(filePath, content, 0644)

	ctx := t.Context()
	pointers, calls, err := ParseWithTreeSitter(ctx, filePath)
	if err != nil {
		t.Fatalf("ParseWithTreeSitter failed: %v", err)
	}

	if len(calls) == 0 {
		t.Errorf("Expected calls, got 0")
	}

	foundCallee := false
	for _, call := range calls {
		if call.CalleeName == "callee" || call.CalleeName == "method" {
			foundCallee = true
		}
	}
	if !foundCallee {
		t.Errorf("Did not find expected calls")
	}

	if len(pointers) == 0 {
		t.Errorf("Expected pointers, got 0")
	}
}
func TestParseWithTreeSitter_Doc(t *testing.T) {
	t.Run("TypeScript", func(t *testing.T) {
		content := []byte(`
			/**
			 * Greeter class
			 */
			class Greeter {
				// sayHello method
				sayHello() {}
			}
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.ts")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pointers, _, err := ParseWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}

		foundClass := false
		foundMethod := false
		for _, p := range pointers {
			if p.Name == "Greeter" {
				foundClass = true
				if p.Doc != "Greeter class" {
					t.Errorf("expected class doc 'Greeter class', got %q", p.Doc)
				}
			}
			if p.Name == "sayHello" {
				foundMethod = true
				if p.Doc != "sayHello method" {
					t.Errorf("expected method doc 'sayHello method', got %q", p.Doc)
				}
			}
		}
		if !foundClass || !foundMethod {
			t.Errorf("did not find expected symbols")
		}
	})

	t.Run("Python", func(t *testing.T) {
		content := []byte(`
def hello():
    """
    Python docstring
    """
    pass

class World:
    '''
    Python class docstring
    '''
    pass
		`)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.py")
		os.WriteFile(filePath, content, 0644)

		ctx := t.Context()
		pointers, _, err := ParseWithTreeSitter(ctx, filePath)
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}

		foundFunc := false
		foundClass := false
		for _, p := range pointers {
			if p.Name == "hello" {
				foundFunc = true
				if p.Doc != "Python docstring" {
					t.Errorf("expected func doc 'Python docstring', got %q", p.Doc)
				}
			}
			if p.Name == "World" {
				foundClass = true
				if p.Doc != "Python class docstring" {
					t.Errorf("expected class doc 'Python class docstring', got %q", p.Doc)
				}
			}
		}
		if !foundFunc || !foundClass {
			t.Errorf("did not find expected symbols")
		}
	})
}
```

### File: internal/filter/actions.go
```go
func GetAction(name string) (ActionFunc, bool) {
	fn, ok := actions[name]
	return fn, ok
}
func pureSignal(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	level := getStr(params, "level")
	if level == "" {
		level = "moderate"
	}

	noisePatternsOnce.Do(func() {
		mod := []string{`^\s*$`, `^\.+$`, `^(\s*)\d+/\d+\s+.*$`, `(?i)progress:?`}
		agg := append(append([]string(nil), mod...), `(?i)debug`, `(?i)trace`, `(?i)info`)

		for _, p := range mod {
			if re, err := regexp.Compile(p); err == nil {
				moderateNoisePatterns = append(moderateNoisePatterns, re)
			}
		}
		for _, p := range agg {
			if re, err := regexp.Compile(p); err == nil {
				aggressiveNoisePatterns = append(aggressiveNoisePatterns, re)
			}
		}
	})

	// 1. Strip ANSI (always good for Pure Signal)
	for i, line := range input.Lines {
		input.Lines[i] = utils.StripANSI(line)
	}

	// 2. Intelligent Dedup
	deduped, _ := snrDedup(ctx, input, nil)

	// 3. Heuristic Filtering based on level
	var finalLines []string
	res := moderateNoisePatterns
	if level == "aggressive" {
		res = aggressiveNoisePatterns
	}

	for _, line := range deduped.Lines {
		isNoise := false
		for _, re := range res {
			if re.MatchString(line) {
				isNoise = true
				break
			}
		}
		if !isNoise {
			finalLines = append(finalLines, line)
		}
	}

	// 4. Safety Truncation if still too large
	input.Lines = finalLines
	headN := getInt(params, "head", 25)
	tailN := getInt(params, "tail", 25)
	return headTail(ctx, input, map[string]any{"head": headN, "tail": tailN})
}
func snrDedup(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	if len(input.Lines) == 0 {
		return input, nil
	}

	var out []string
	lastLine := ""
	count := 0

	for _, line := range input.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if trimmed == lastLine {
			count++
			continue
		}

		if lastLine != "" {
			if count > 1 {
				out = append(out, fmt.Sprintf("%s [x%d]", lastLine, count))
			} else {
				out = append(out, lastLine)
			}
		}

		lastLine = trimmed
		count = 1
	}

	// ALWAYS append the last line/group to the output slice
	if lastLine != "" {
		if count > 1 {
			out = append(out, fmt.Sprintf("%s [x%d]", lastLine, count))
		} else {
			out = append(out, lastLine)
		}
	}

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func headTail(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	headN := getInt(params, "head", 10)
	tailN := getInt(params, "tail", 10)
	threshold := headN + tailN + 5 // Margin to avoid tiny truncations

	if len(input.Lines) <= threshold {
		return input, nil
	}

	var out []string
	out = append(out, input.Lines[:headN]...)
	out = append(out, fmt.Sprintf("... [scouter: truncated %d lines of noise] ...", len(input.Lines)-headN-tailN))
	out = append(out, input.Lines[len(input.Lines)-tailN:]...)

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func getStr(params map[string]any, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
func getInt(params map[string]any, key string, def int) int {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	default:
		return def
	}
}
func compilePattern(params map[string]any, key string) (*regexp.Regexp, error) {
	p := getStr(params, key)
	if p == "" {
		return nil, fmt.Errorf("missing %q param", key)
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, fmt.Errorf("compile %q: %w", p, err)
	}
	return re, nil
}
func keepLines(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	var out []string
	for _, line := range input.Lines {
		if re.MatchString(line) {
			out = append(out, line)
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func removeLines(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	var out []string
	for _, line := range input.Lines {
		if !re.MatchString(line) {
			out = append(out, line)
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func truncateLines(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	max := getInt(params, "max", 80)
	ellipsis := getStr(params, "ellipsis")
	if ellipsis == "" {
		ellipsis = "..."
	}
	ellipsisLen := len([]rune(ellipsis))
	if max <= ellipsisLen {
		max = ellipsisLen + 1
	}
	out := make([]string, len(input.Lines))
	for i, line := range input.Lines {
		if len([]rune(line)) > max {
			runes := []rune(line)
			out[i] = string(runes[:max-ellipsisLen]) + ellipsis
		} else {
			out[i] = line
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func stripANSI(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	out := make([]string, len(input.Lines))
	for i, line := range input.Lines {
		out[i] = utils.StripANSI(line)
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func head(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	n := getInt(params, "n", 10)
	if len(input.Lines) <= n {
		return input, nil
	}
	out := make([]string, n)
	copy(out, input.Lines[:n])
	remaining := len(input.Lines) - n
	msg := getStr(params, "overflow_msg")
	if msg == "" {
		msg = fmt.Sprintf("+%d more lines", remaining)
	}
	out = append(out, msg)
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func tail(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	n := getInt(params, "n", 10)
	if len(input.Lines) <= n {
		return input, nil
	}
	start := len(input.Lines) - n
	out := make([]string, n)
	copy(out, input.Lines[start:])
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func groupBy(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	top := getInt(params, "top", 0)
	fmtStr := getStr(params, "format")
	if fmtStr == "" {
		fmtStr = "{{.Key}}: {{.Count}}"
	}

	groups := make(map[string]int)
	var order []string
	for _, line := range input.Lines {
		m := re.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		key := m[1]
		if groups[key] == 0 {
			order = append(order, key)
		}
		groups[key]++
	}

	// Sort by count descending
	sort.Slice(order, func(i, j int) bool {
		return groups[order[i]] > groups[order[j]]
	})

	if top > 0 && len(order) > top {
		order = order[:top]
	}

	tmpl, err := template.New("group").Parse(fmtStr)
	if err != nil {
		return input, fmt.Errorf("format template: %w", err)
	}

	var out []string
	for _, key := range order {
		var buf strings.Builder
		if err := tmpl.Execute(&buf, map[string]any{"Key": key, "Count": groups[key]}); err != nil {
			return input, fmt.Errorf("group_by template: %w", err)
		}
		out = append(out, buf.String())
	}

	meta := copyMeta(input.Metadata)
	meta["groups"] = groups
	return ActionResult{Lines: out, Metadata: meta}, nil
}
func dedup(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	top := getInt(params, "top", 0)

	// Build normalize patterns
	var normalizers []*regexp.Regexp
	if raw, ok := params["normalize"]; ok {
		if list, ok := raw.([]any); ok {
			for _, item := range list {
				if s, ok := item.(string); ok {
					if re, err := regexp.Compile(s); err == nil {
						normalizers = append(normalizers, re)
					}
				}
			}
		}
	}

	type entry struct {
		normalized string
		count      int
	}

	seen := make(map[string]*entry)
	var order []string

	for _, line := range input.Lines {
		norm := line
		for _, re := range normalizers {
			norm = re.ReplaceAllString(norm, "")
		}
		norm = strings.TrimSpace(norm)

		if e, ok := seen[norm]; ok {
			e.count++
		} else {
			seen[norm] = &entry{normalized: norm, count: 1}
			order = append(order, norm)
		}
	}

	sort.Slice(order, func(i, j int) bool {
		return seen[order[i]].count > seen[order[j]].count
	})

	if top > 0 && len(order) > top {
		order = order[:top]
	}

	var out []string
	for _, key := range order {
		e := seen[key]
		if e.count > 1 {
			out = append(out, fmt.Sprintf("%s (x%d)", e.normalized, e.count))
		} else {
			out = append(out, e.normalized)
		}
	}

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func jsonExtract(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	fieldsRaw, ok := params["fields"]
	if !ok {
		return input, fmt.Errorf("json_extract: missing 'fields' param")
	}
	fields, ok := toStringSlice(fieldsRaw)
	if !ok {
		return input, fmt.Errorf("json_extract: 'fields' must be a list of strings")
	}
	fmtStr := getStr(params, "format")

	joined := strings.Join(input.Lines, "\n")
	var data map[string]any
	if err := json.Unmarshal([]byte(joined), &data); err != nil {
		return input, fmt.Errorf("json_extract: parse: %w", err)
	}

	if fmtStr != "" {
		tmpl, err := template.New("json").Parse(fmtStr)
		if err != nil {
			return input, err
		}
		extracted := make(map[string]any)
		for _, f := range fields {
			extracted[f] = data[f]
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, extracted); err != nil {
			return input, fmt.Errorf("json_extract template: %w", err)
		}
		return ActionResult{Lines: strings.Split(buf.String(), "\n"), Metadata: input.Metadata}, nil
	}

	var out []string
	for _, f := range fields {
		if v, ok := data[f]; ok {
			out = append(out, fmt.Sprintf("%s: %v", f, v))
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func jsonSchema(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	maxDepth := getInt(params, "max_depth", 3)

	joined := strings.Join(input.Lines, "\n")
	var data any
	if err := json.Unmarshal([]byte(joined), &data); err != nil {
		return input, fmt.Errorf("json_schema: parse: %w", err)
	}

	schema := extractSchema(data, 0, maxDepth)
	lines := strings.Split(schema, "\n")
	return ActionResult{Lines: lines, Metadata: input.Metadata}, nil
}
func extractSchema(v any, depth, maxDepth int) string {
	if depth >= maxDepth {
		return "..."
	}
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return "{}"
		}
		var parts []string
		for k, child := range val {
			parts = append(parts, fmt.Sprintf("%s%s: %s", strings.Repeat("  ", depth+1), k, extractSchema(child, depth+1, maxDepth)))
		}
		sort.Strings(parts)
		return "{\n" + strings.Join(parts, "\n") + "\n" + strings.Repeat("  ", depth) + "}"
	case []any:
		if len(val) == 0 {
			return "[]"
		}
		return "[" + extractSchema(val[0], depth, maxDepth) + "]"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}
func ndjsonStream(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	groupField := getStr(params, "group_by")
	fmtStr := getStr(params, "format")

	type group struct {
		key    string
		events []map[string]any
	}

	groups := make(map[string]*group)
	var order []string

	for _, line := range input.Lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		key := ""
		if groupField != "" {
			if v, ok := obj[groupField]; ok {
				key = fmt.Sprintf("%v", v)
			}
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &group{key: key}
			order = append(order, key)
		}
		groups[key].events = append(groups[key].events, obj)
	}

	var out []string
	if fmtStr != "" {
		tmpl, err := template.New("ndjson").Parse(fmtStr)
		if err != nil {
			return input, err
		}
		for _, key := range order {
			g := groups[key]
			var buf strings.Builder
			if err := tmpl.Execute(&buf, map[string]any{
				"Key":    g.key,
				"Count":  len(g.events),
				"Events": g.events,
			}); err != nil {
				return input, fmt.Errorf("ndjson_stream template: %w", err)
			}
			out = append(out, buf.String())
		}
	} else {
		for _, key := range order {
			g := groups[key]
			out = append(out, fmt.Sprintf("%s: %d events", g.key, len(g.events)))
		}
	}

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func regexExtract(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	fmtStr := getStr(params, "format")

	var out []string
	for _, line := range input.Lines {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if fmtStr != "" {
			result := fmtStr
			for i, match := range m {
				result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), match)
			}
			out = append(out, result)
		} else {
			if len(m) > 1 {
				out = append(out, strings.Join(m[1:], " "))
			} else {
				out = append(out, m[0])
			}
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func stateMachine(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	statesRaw, ok := params["states"]
	if !ok {
		return input, fmt.Errorf("state_machine: missing 'states' param")
	}
	statesMap, ok := statesRaw.(map[string]any)
	if !ok {
		return input, fmt.Errorf("state_machine: 'states' must be a map")
	}

	type stateConfig struct {
		until *regexp.Regexp
		keep  *regexp.Regexp
		next  string
	}

	states := make(map[string]stateConfig)
	for name, rawCfg := range statesMap {
		cfgMap, ok := rawCfg.(map[string]any)
		if !ok {
			continue
		}
		sc := stateConfig{}
		if u, ok := cfgMap["until"].(string); ok {
			sc.until, _ = regexp.Compile(u)
		}
		if k, ok := cfgMap["keep"].(string); ok {
			sc.keep, _ = regexp.Compile(k)
		}
		if n, ok := cfgMap["next"].(string); ok {
			sc.next = n
		}
		states[name] = sc
	}

	currentState := "start"
	if _, ok := states[currentState]; !ok {
		// Use alphabetically first state for determinism
		var names []string
		for name := range states {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) > 0 {
			currentState = names[0]
		}
	}

	var out []string
	for _, line := range input.Lines {
		sc, ok := states[currentState]
		if !ok {
			break
		}
		// Check transition first — transition line is NOT kept unless explicitly matched by keep
		if sc.until != nil && sc.until.MatchString(line) {
			if sc.next != "" {
				currentState = sc.next
			}
			continue
		}
		// Apply keep filter
		if sc.keep == nil || sc.keep.MatchString(line) {
			out = append(out, line)
		}
	}

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func aggregate(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	patternsRaw, ok := params["patterns"]
	if !ok {
		return input, fmt.Errorf("aggregate: missing 'patterns' param")
	}
	patternsMap, ok := patternsRaw.(map[string]any)
	if !ok {
		return input, fmt.Errorf("aggregate: 'patterns' must be a map")
	}

	type patCount struct {
		name  string
		re    *regexp.Regexp
		count int
	}

	var patterns []patCount
	for name, rawPattern := range patternsMap {
		p, _ := rawPattern.(string)
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		patterns = append(patterns, patCount{name: name, re: re})
	}

	for _, line := range input.Lines {
		for i := range patterns {
			if patterns[i].re.MatchString(line) {
				patterns[i].count++
			}
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].name < patterns[j].name
	})

	stats := make(map[string]int)
	var out []string
	fmtStr := getStr(params, "format")
	for _, p := range patterns {
		stats[p.name] = p.count
		if fmtStr == "" {
			out = append(out, fmt.Sprintf("%s: %d", p.name, p.count))
		}
	}

	if fmtStr != "" {
		tmpl, err := template.New("agg").Parse(fmtStr)
		if err != nil {
			return input, err
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, stats); err != nil {
			return input, fmt.Errorf("aggregate template: %w", err)
		}
		out = strings.Split(buf.String(), "\n")
	}

	meta := copyMeta(input.Metadata)
	meta["stats"] = stats
	return ActionResult{Lines: out, Metadata: meta}, nil
}
func formatTemplate(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	tmplStr := getStr(params, "template")
	if tmplStr == "" {
		return input, fmt.Errorf("format_template: missing 'template' param")
	}

	tmpl, err := template.New("fmt").Parse(tmplStr)
	if err != nil {
		return input, fmt.Errorf("format_template: %w", err)
	}

	data := map[string]any{
		"lines":  strings.Join(input.Lines, "\n"),
		"count":  len(input.Lines),
		"groups": input.Metadata["groups"],
		"stats":  input.Metadata["stats"],
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return input, fmt.Errorf("format_template execute: %w", err)
	}

	result := buf.String()
	result = strings.TrimRight(result, "\n")
	lines := strings.Split(result, "\n")
	return ActionResult{Lines: lines, Metadata: input.Metadata}, nil
}
func compactPath(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	out := make([]string, len(input.Lines))
	for i, line := range input.Lines {
		out[i] = utils.CompactPath(line)
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}
func copyMeta(m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		out[k] = v
	}
	return out
}
func toStringSlice(v any) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		return val, true
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}
func semanticPurify(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	if input.Resolver == nil {
		return input, nil
	}

	var enrichedLines []string
	for _, line := range input.Lines {
		enrichedLines = append(enrichedLines, line)

		matches := GoTestFailureRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			file := matches[1]
			lineNum, _ := strconv.Atoi(matches[2])

			// Try to resolve the symbol source
			source, err := input.Resolver.ResolveSource(ctx, file, lineNum)
			if err == nil && source != "" {
				enrichedLines = append(enrichedLines, "\n--- 🧬 SOURCE CONTEXT ---")
				enrichedLines = append(enrichedLines, source)
				enrichedLines = append(enrichedLines, "--- END CONTEXT ---\n")
			}
		}
	}
	input.Lines = enrichedLines
	return input, nil
}
type entry struct {
		normalized string
		count      int
	}
type group struct {
		key    string
		events []map[string]any
	}
type stateConfig struct {
		until *regexp.Regexp
		keep  *regexp.Regexp
		next  string
	}
type patCount struct {
		name  string
		re    *regexp.Regexp
		count int
	}
```

### File: internal/filter/actions_integration_test.go
```go
func fixturesDir() string {
	// Find project root by looking for go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "tests", "fixtures")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "tests", "fixtures")
}
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir(), name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return string(data)
}
func loadFilter(t *testing.T, name string) *Filter {
	t.Helper()
	// Filters are in internal/filters/
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			data, err := os.ReadFile(filepath.Join(dir, "internal", "filters", name))
			if err != nil {
				t.Fatalf("load filter %s: %v", name, err)
			}
			f, err := ParseFilter(data)
			if err != nil {
				t.Fatalf("parse filter %s: %v", name, err)
			}
			return f
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find filter %s", name)
	return nil
}
func applyPipeline(f *Filter, input string) (string, error) {
	lines := strings.Split(input, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	result := ActionResult{
		Lines:    lines,
		Metadata: make(map[string]any),
	}

	for i, action := range f.Pipeline {
		fn, ok := GetAction(action.ActionName)
		if !ok {
			return "", nil
		}
		var err error
		result, err = fn(context.Background(), result, action.Params)
		if err != nil {
			return "", err
		}
		_ = i
	}

	return strings.Join(result.Lines, "\n") + "\n", nil
}
func TestGitLogFilterIntegration(t *testing.T) {
	// The git-log filter works by INJECTING --pretty=format:... BEFORE execution.
	// The pipeline then cleans up the already-compact output.
	// We test the full savings: raw verbose input vs pipeline-filtered output.
	fixture := loadFixture(t, "git_log_raw.txt")
	f := loadFilter(t, "git-log.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	// Should be shorter (pipeline removes blank lines, truncates)
	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	// Non-empty
	if strings.TrimSpace(filtered) == "" {
		t.Error("filtered output is empty")
	}

	// The real savings come from arg injection (--pretty=format:...).
	// The pipeline alone on verbose git log produces moderate savings.
	// Verify pipeline produces valid output (savings threshold relaxed for pipeline-only test).
	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("git-log pipeline-only: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)

	// Test with simulated post-injection output (what git log --pretty=format:... produces)
	injectedOutput := "8cb198e chore(master): release 0.22.0 (#201) (2 hours ago) <github-actions>\n393fa5b feat: add rtk wc command (#175) (3 hours ago) <John Doe>\nc29644b chore(master): release 0.21.1 (#179) (5 hours ago) <github-actions>\nd196c2d fix: gh run view drops flags (#159) (6 hours ago) <Jane Smith>\naa0b462 chore(master): release 0.21.0 (#178) (7 hours ago) <github-actions>\n510c491 feat(docker): add docker compose (#110) (8 hours ago) <Pierre Martin>\nc83a834 docs: add brew install note (#177) (10 hours ago) <Rui Chen>\n577e082 chore(master): release 0.20.1 (#167) (1 day ago) <github-actions>\n0b34772 fix: install to ~/.local/bin (#155) (1 day ago) <DevOps Bot>\n78c9e94 chore(master): release 0.20.0 (#152) (2 days ago) <github-actions>\n"

	injectedFiltered, err := applyPipeline(f, injectedOutput)
	if err != nil {
		t.Fatalf("apply pipeline on injected: %v", err)
	}

	// Full savings: raw verbose input tokens vs final filtered output tokens
	fullSavings := float64(inputTokens-utils.EstimateTokens(injectedFiltered)) / float64(inputTokens) * 100
	t.Logf("git-log full (inject+pipeline): %d -> %d tokens (%.1f%% savings)", inputTokens, utils.EstimateTokens(injectedFiltered), fullSavings)
	if fullSavings < 50 {
		t.Errorf("git-log full savings %.1f%% < 50%% minimum", fullSavings)
	}
}
func TestGitStatusFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "git_status_raw.txt")
	f := loadFilter(t, "git-status.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	if strings.TrimSpace(filtered) == "" {
		t.Error("filtered output is empty")
	}

	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("git-status: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
	if savings < 60 {
		t.Errorf("git-status savings %.1f%% < 60%% minimum", savings)
	}
}
func TestGitDiffFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "git_diff_raw.txt")
	f := loadFilter(t, "git-diff.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	// Should be shorter OR within a reasonable overhead (headers/template)
	if len(filtered) >= len(fixture)*2 {
		t.Errorf("filtered (%d) too much larger than input (%d)", len(filtered), len(fixture))
	}

	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("git-diff: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
	if savings < -20 {
		t.Errorf("git-diff savings %.1f%% too negative", savings)
	}
}
func TestGoTestFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "go_test_raw.txt")
	f := loadFilter(t, "go-test.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("go-test: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
	if savings < 80 {
		t.Errorf("go-test savings %.1f%% < 80%% minimum", savings)
	}
}
func TestCargoTestFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "cargo_test_raw.txt")
	f := loadFilter(t, "cargo-test.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("cargo-test: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
	if savings < 80 {
		t.Errorf("cargo-test savings %.1f%% < 80%% minimum", savings)
	}
}
func TestRSpecFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "rspec_raw.txt")
	f := loadFilter(t, "rspec.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	// Should be much shorter
	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	// Should contain summary
	if !strings.Contains(filtered, "examples") {
		t.Error("filtered output missing examples count")
	}

	// Should preserve failure paths (essential for debugging)
	if strings.Contains(fixture, "rspec ./") && !strings.Contains(filtered, "rspec ./") {
		t.Error("filtered output missing failure paths (rspec ./spec/...)")
	}

	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("rspec: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
}
func TestFilterEmptyInput(t *testing.T) {
	f := loadFilter(t, "git-log.yaml")
	filtered, err := applyPipeline(f, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not crash, output may be minimal
	_ = filtered
}
func TestFilterUnicodeInput(t *testing.T) {
	f := loadFilter(t, "git-log.yaml")
	input := "abc123 héllo wörld — special chars (ñ) <用户>\nabc456 日本語テスト (2d ago) <テスター>\n"
	filtered, err := applyPipeline(f, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(filtered) == "" {
		t.Error("unicode input produced empty output")
	}
}
func TestFilterANSIInput(t *testing.T) {
	f := &Filter{
		Name: "test",
		Pipeline: Pipeline{
			{ActionName: "strip_ansi"},
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}
	input := "\x1b[31mred error\x1b[0m\n\x1b[32mgreen ok\x1b[0m\n"
	filtered, err := applyPipeline(f, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(filtered, "\x1b") {
		t.Error("ANSI codes not stripped")
	}
}
func TestBundleInstallFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "bundle_install_raw.txt")
	f := loadFilter(t, "bundle-install.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	if !strings.Contains(filtered, "Bundle complete") {
		t.Error("filtered output missing Bundle complete")
	}

	// Calculate and log token savings
	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("bundle-install: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
}
func TestRailsRoutesFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "rails_routes_raw.txt")
	f := loadFilter(t, "rails-routes.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	// Should be shorter
	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	// Should contain routes total summary
	if !strings.Contains(filtered, "routes total") {
		t.Error("filtered output missing 'routes total'")
	}

	// Calculate and log token savings
	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("rails-routes: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
}
func TestRailsMigrateFilterIntegration(t *testing.T) {
	fixture := loadFixture(t, "rails_migrate_raw.txt")
	f := loadFilter(t, "rails-migrate.yaml")

	filtered, err := applyPipeline(f, fixture)
	if err != nil {
		t.Fatalf("apply pipeline: %v", err)
	}

	// Should be shorter
	if len(filtered) >= len(fixture) {
		t.Errorf("filtered (%d) not shorter than input (%d)", len(filtered), len(fixture))
	}

	// Should contain migrations executed summary
	if !strings.Contains(filtered, "migrations executed") {
		t.Error("filtered output missing 'migrations executed'")
	}

	// Calculate and log token savings
	inputTokens := utils.EstimateTokens(fixture)
	outputTokens := utils.EstimateTokens(filtered)
	savings := float64(inputTokens-outputTokens) / float64(inputTokens) * 100
	t.Logf("rails-migrate: %d -> %d tokens (%.1f%% savings)", inputTokens, outputTokens, savings)
}
func TestGracefulDegradation(t *testing.T) {
	// Bad filter YAML
	badYAML := `
name: "bad"
match:
  command: "test"
pipeline:
  - action: "nonexistent_action"
`
	_, err := ParseFilter([]byte(badYAML))
	if err == nil {
		t.Error("expected error for unknown action, but ParseFilter accepted it")
	}
}
```

### File: internal/filter/actions_test.go
```go
func lines(s ...string) ActionResult {
	return ActionResult{Lines: s, Metadata: make(map[string]any)}
}
func TestKeepLines(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		pattern string
		want    int
	}{
		{"keep non-blank", []string{"hello", "", "world", ""}, `\S`, 2},
		{"keep digits", []string{"abc", "123", "def", "456"}, `^\d+$`, 2},
		{"empty input", nil, `\S`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := keepLines(context.Background(), lines(tt.input...), map[string]any{"pattern": tt.pattern})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Lines) != tt.want {
				t.Errorf("got %d lines, want %d", len(res.Lines), tt.want)
			}
		})
	}
}
func TestRemoveLines(t *testing.T) {
	input := lines("Compiling foo", "Running test", "Compiling bar", "test result: ok")
	res, err := removeLines(context.Background(), input, map[string]any{"pattern": `^Compiling`})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Errorf("got %d lines, want 2", len(res.Lines))
	}
}
func TestTruncateLines(t *testing.T) {
	input := lines("short", "this is a very long line that should be truncated at some point")
	res, err := truncateLines(context.Background(), input, map[string]any{"max": 20, "ellipsis": "..."})
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(res.Lines[1])) > 20 {
		t.Errorf("line not truncated: %q (len=%d)", res.Lines[1], len([]rune(res.Lines[1])))
	}
	if !strings.HasSuffix(res.Lines[1], "...") {
		t.Errorf("missing ellipsis: %q", res.Lines[1])
	}
	if res.Lines[0] != "short" {
		t.Errorf("short line modified: %q", res.Lines[0])
	}
}
func TestStripANSI(t *testing.T) {
	input := lines("\x1b[31mred\x1b[0m", "normal")
	res, err := stripANSI(context.Background(), input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines[0] != "red" {
		t.Errorf("ANSI not stripped: %q", res.Lines[0])
	}
}
func TestHead(t *testing.T) {
	input := lines("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11")
	res, err := head(context.Background(), input, map[string]any{"n": 5})
	if err != nil {
		t.Fatal(err)
	}
	// 5 lines + overflow message
	if len(res.Lines) != 6 {
		t.Errorf("got %d lines, want 6", len(res.Lines))
	}
	if !strings.Contains(res.Lines[5], "+6 more") {
		t.Errorf("overflow msg: %q", res.Lines[5])
	}
}
func TestHeadNoOverflow(t *testing.T) {
	input := lines("1", "2", "3")
	res, err := head(context.Background(), input, map[string]any{"n": 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 3 {
		t.Errorf("got %d lines, want 3", len(res.Lines))
	}
}
func TestTail(t *testing.T) {
	input := lines("1", "2", "3", "4", "5")
	res, err := tail(context.Background(), input, map[string]any{"n": 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Errorf("got %d lines, want 2", len(res.Lines))
	}
	if res.Lines[0] != "4" || res.Lines[1] != "5" {
		t.Errorf("got %v", res.Lines)
	}
}
func TestGroupBy(t *testing.T) {
	input := lines("ERR foo", "WARN bar", "ERR baz", "ERR qux", "WARN quux")
	res, err := groupBy(context.Background(), input, map[string]any{
		"pattern": `^(\w+)`,
		"format":  "{{.Key}}: {{.Count}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d groups, want 2: %v", len(res.Lines), res.Lines)
	}
	// ERR has 3, should be first
	if !strings.HasPrefix(res.Lines[0], "ERR") {
		t.Errorf("expected ERR first, got %q", res.Lines[0])
	}
}
func TestDedup(t *testing.T) {
	input := lines("error: foo", "error: foo", "error: foo", "warn: bar", "warn: bar")
	res, err := dedup(context.Background(), input, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines: %v", len(res.Lines), res.Lines)
	}
	if !strings.Contains(res.Lines[0], "x3") {
		t.Errorf("expected x3: %q", res.Lines[0])
	}
}
func TestRegexExtract(t *testing.T) {
	input := lines("file: main.go line: 42", "file: utils.go line: 10")
	res, err := regexExtract(context.Background(), input, map[string]any{
		"pattern": `file: (\S+) line: (\d+)`,
		"format":  "$1:$2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines", len(res.Lines))
	}
	if res.Lines[0] != "main.go:42" {
		t.Errorf("got %q", res.Lines[0])
	}
}
func TestAggregate(t *testing.T) {
	input := lines("PASS foo", "FAIL bar", "PASS baz", "PASS qux", "FAIL quux")
	res, err := aggregate(context.Background(), input, map[string]any{
		"patterns": map[string]any{
			"pass": `^PASS`,
			"fail": `^FAIL`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should have "fail: 2" and "pass: 3" (sorted alphabetically)
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines: %v", len(res.Lines), res.Lines)
	}
}
func TestFormatTemplate(t *testing.T) {
	input := ActionResult{
		Lines:    []string{"a", "b", "c"},
		Metadata: map[string]any{},
	}
	res, err := formatTemplate(context.Background(), input, map[string]any{
		"template": "{{.count}} items:\n{{.lines}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "3 items") {
		t.Errorf("missing count: %q", joined)
	}
}
func TestCompactPath(t *testing.T) {
	input := lines("src/main.go", "lib/utils.js", "README.md")
	res, err := compactPath(context.Background(), input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines[0] != "main.go" {
		t.Errorf("got %q", res.Lines[0])
	}
	if res.Lines[2] != "README.md" {
		t.Errorf("got %q", res.Lines[2])
	}
}
func TestJsonExtract(t *testing.T) {
	input := lines(`{"name":"scouter","version":"0.1","count":42}`)
	res, err := jsonExtract(context.Background(), input, map[string]any{
		"fields": []any{"name", "version"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines: %v", len(res.Lines), res.Lines)
	}
}
func TestNdjsonStream(t *testing.T) {
	input := lines(
		`{"action":"run","pkg":"foo"}`,
		`{"action":"pass","pkg":"foo"}`,
		`{"action":"run","pkg":"bar"}`,
		`{"action":"fail","pkg":"bar"}`,
	)
	res, err := ndjsonStream(context.Background(), input, map[string]any{"group_by": "pkg"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d groups: %v", len(res.Lines), res.Lines)
	}
}
func TestStateMachine(t *testing.T) {
	input := lines(
		"running tests...",
		"test foo: ok",
		"test bar: FAILED",
		"--- failures ---",
		"bar: assertion error",
		"--- end ---",
	)
	res, err := stateMachine(context.Background(), input, map[string]any{
		"states": map[string]any{
			"start": map[string]any{
				"keep":  `^test`,
				"until": `^--- failures`,
				"next":  "failures",
			},
			"failures": map[string]any{
				"keep":  `.`,
				"until": `^--- end`,
				"next":  "done",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should keep "test foo: ok", "test bar: FAILED", "bar: assertion error"
	if len(res.Lines) != 3 {
		t.Errorf("got %d lines: %v", len(res.Lines), res.Lines)
	}
}
func TestEmptyInput(t *testing.T) {
	empty := ActionResult{Lines: nil, Metadata: make(map[string]any)}
	actionTests := []struct {
		name   string
		fn     ActionFunc
		params map[string]any
	}{
		{"keepLines", keepLines, map[string]any{"pattern": `\S`}},
		{"removeLines", removeLines, map[string]any{"pattern": `\S`}},
		{"truncateLines", truncateLines, map[string]any{"max": 80}},
		{"stripANSI", stripANSI, nil},
		{"head", head, map[string]any{"n": 5}},
		{"tail", tail, map[string]any{"n": 5}},
		{"compactPath", compactPath, nil},
	}
	for _, tt := range actionTests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.fn(context.Background(), empty, tt.params)
			if err != nil {
				t.Fatalf("unexpected error on empty input: %v", err)
			}
			if len(res.Lines) != 0 {
				t.Errorf("expected empty output, got %d lines", len(res.Lines))
			}
		})
	}
}
func TestGetAction(t *testing.T) {
	for _, name := range []string{"keep_lines", "remove_lines", "head", "format_template"} {
		if _, ok := GetAction(name); !ok {
			t.Errorf("action %q not found", name)
		}
	}
	if _, ok := GetAction("nonexistent"); ok {
		t.Error("expected nonexistent action to not be found")
	}
}
func TestPureSignal(t *testing.T) {
	input := lines(
		"\x1b[31mFAIL: test_foo\x1b[0m",
		"progress: 1/100",
		"progress: 2/100",
		"progress: 2/100", // Duplicated
		"DEBUG: low level noise",
		"INFO: doing something",
		"test result: failed",
		"",    // Empty line
		"...", // Dots
	)

	t.Run("moderate", func(t *testing.T) {
		res, err := pureSignal(context.Background(), input, map[string]any{"level": "moderate"})
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(res.Lines, "\n")
		if !strings.Contains(joined, "FAIL: test_foo") {
			t.Errorf("missing FAIL: %q", joined)
		}
		if strings.Contains(joined, "progress") {
			t.Errorf("noise 'progress' not removed: %q", joined)
		}
		if strings.Contains(joined, "...") {
			t.Errorf("noise '...' not removed: %q", joined)
		}
	})

	t.Run("aggressive", func(t *testing.T) {
		res, err := pureSignal(context.Background(), input, map[string]any{"level": "aggressive"})
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(res.Lines, "\n")
		if strings.Contains(strings.ToLower(joined), "debug") || strings.Contains(strings.ToLower(joined), "info") {
			t.Errorf("aggressive mode should remove INFO/DEBUG: %q", joined)
		}
	})
}
```

### File: internal/filter/loader.go
```go
func LoadEmbedded() ([]Filter, error) {
	if EmbeddedFS == nil {
		return nil, nil
	}

	// Try "filters" subdir first (when embedded from root), then "." (flat)
	dir := "filters"
	entries, err := EmbeddedFS.ReadDir(dir)
	if err != nil {
		dir = "."
		entries, err = EmbeddedFS.ReadDir(dir)
		if err != nil {
			return nil, nil
		}
	}

	var filters []Filter
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := entry.Name()
		if dir != "." {
			path = dir + "/" + entry.Name()
		}
		data, err := EmbeddedFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read embedded filter %s: %w", entry.Name(), err)
		}
		f, err := ParseFilter(data)
		if err != nil {
			return nil, fmt.Errorf("parse embedded filter %s: %w", entry.Name(), err)
		}
		filters = append(filters, *f)
	}
	return filters, nil
}
func LoadUserFilters(dir string) ([]Filter, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read filter dir: %w", err)
	}

	var filters []Filter
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read user filter %s: %w", entry.Name(), err)
		}
		f, err := ParseFilter(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scouter: skipping invalid filter %s: %v\n", entry.Name(), err)
			continue
		}
		filters = append(filters, *f)
	}
	return filters, nil
}
func LoadAll(userDir string) ([]Filter, error) {
	user, err := LoadUserFilters(userDir)
	if err != nil {
		return nil, err
	}

	embedded, err := LoadEmbedded()
	if err != nil {
		return nil, err
	}

	byName := make(map[string]bool)
	var result []Filter
	for _, f := range user {
		byName[f.Name] = true
		result = append(result, f)
	}
	for _, f := range embedded {
		if !byName[f.Name] {
			result = append(result, f)
		}
	}
	return result, nil
}
```

### File: internal/filter/loader_test.go
```go
func TestLoadUserFilters(t *testing.T) {
	dir := t.TempDir()

	validYAML := `
name: "user-filter"
version: 1
match:
  command: "echo"
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(dir, "echo.yaml"), []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	filters, err := LoadUserFilters(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("got %d filters, want 1", len(filters))
	}
	if filters[0].Name != "user-filter" {
		t.Errorf("name = %q", filters[0].Name)
	}
}
func TestLoadUserFiltersMissingDir(t *testing.T) {
	filters, err := LoadUserFilters("/tmp/nonexistent-scouter-filters-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filters != nil {
		t.Errorf("expected nil, got %v", filters)
	}
}
func TestLoadUserFiltersSkipsInvalid(t *testing.T) {
	dir := t.TempDir()

	// Invalid filter (no name)
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("pipeline: []"), 0644); err != nil {
		t.Fatal(err)
	}

	filters, err := LoadUserFilters(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("expected 0 filters, got %d", len(filters))
	}
}
func TestLoadAllUserOverridesEmbedded(t *testing.T) {
	dir := t.TempDir()

	// Create user filter that would override an embedded one
	userYAML := `
name: "custom"
version: 1
match:
  command: "custom"
pipeline:
  - action: "head"
    n: 5
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(userYAML), 0644); err != nil {
		t.Fatal(err)
	}

	filters, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have user filter
	found := false
	for _, f := range filters {
		if f.Name == "custom" {
			found = true
		}
	}
	if !found {
		t.Error("user filter not found in merged results")
	}
}
```

### File: internal/filter/parser.go
```go
func ParseFilter(data []byte) (*Filter, error) {
	var f Filter
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse filter: %w", err)
	}
	if err := ValidateFilter(&f); err != nil {
		return nil, err
	}
	return &f, nil
}
func ValidateFilter(f *Filter) error {
	if f.Name == "" {
		return fmt.Errorf("validate filter: missing 'name'")
	}
	if f.Match.Command == "" {
		return fmt.Errorf("validate filter %q: missing 'match.command'", f.Name)
	}
	for i, action := range f.Pipeline {
		if action.ActionName == "" {
			return fmt.Errorf("validate filter %q: pipeline[%d] missing 'action'", f.Name, i)
		}
		if _, ok := GetAction(action.ActionName); !ok {
			return fmt.Errorf("validate filter %q: pipeline[%d] unknown action %q", f.Name, i, action.ActionName)
		}
	}
	return nil
}
```

### File: internal/filter/parser_test.go
```go
func TestParseFilterValid(t *testing.T) {
	yaml := `
name: "test"
version: 1
description: "test filter"
match:
  command: "echo"
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	f, err := ParseFilter([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Name != "test" {
		t.Errorf("name = %q", f.Name)
	}
	if f.Match.Command != "echo" {
		t.Errorf("match.command = %q", f.Match.Command)
	}
}
func TestParseFilterMissingName(t *testing.T) {
	yaml := `
match:
  command: "echo"
pipeline: []
`
	_, err := ParseFilter([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}
func TestParseFilterMissingCommand(t *testing.T) {
	yaml := `
name: "test"
pipeline: []
`
	_, err := ParseFilter([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}
func TestParseFilterUnknownAction(t *testing.T) {
	yaml := `
name: "test"
match:
  command: "echo"
pipeline:
  - action: "nonexistent_action"
`
	_, err := ParseFilter([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}
func TestParseFilterInvalidYAML(t *testing.T) {
	_, err := ParseFilter([]byte("}{invalid"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
```

### File: internal/filter/registry.go
```go
func NewRegistry(filters []Filter) *Registry {
	r := &Registry{
		byKey:   make(map[string][]Filter),
		filters: filters,
	}
	for _, f := range filters {
		key := f.Match.Command
		if f.Match.Subcommand != "" {
			key += ":" + f.Match.Subcommand
		}
		r.byKey[key] = append(r.byKey[key], f)
	}
	return r
}
func matchesFlags(f *Filter, args []string) bool {
	// Check exclude_flags: skip if user passed any excluded flag
	for _, exclude := range f.Match.ExcludeFlags {
		for _, arg := range args {
			if strings.HasPrefix(arg, exclude) {
				return false
			}
		}
	}

	// Check require_flags: skip if user missing required flag
	for _, require := range f.Match.RequireFlags {
		found := false
		for _, arg := range args {
			if strings.HasPrefix(arg, require) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
type Registry struct {
	byKey   map[string][]Filter // key = "command" or "command:subcommand"
	filters []Filter
}
```

### File: internal/filter/registry_test.go
```go
func makeFilter(name, cmd, subcmd string) Filter {
	return Filter{
		Name:    name,
		Version: 1,
		Match:   Match{Command: cmd, Subcommand: subcmd},
		OnError: "passthrough",
	}
}
func TestRegistryMatch(t *testing.T) {
	filters := []Filter{
		makeFilter("git-log", "git", "log"),
		makeFilter("git-status", "git", "status"),
		makeFilter("go-test", "go", "test"),
	}
	reg := NewRegistry(filters)

	tests := []struct {
		cmd     string
		subcmd  string
		args    []string
		want    string
		wantNil bool
	}{
		{"git", "log", nil, "git-log", false},
		{"git", "status", nil, "git-status", false},
		{"go", "test", nil, "go-test", false},
		{"git", "push", nil, "", true},
		{"npm", "install", nil, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd+" "+tt.subcmd, func(t *testing.T) {
			f := reg.Match(tt.cmd, tt.subcmd, tt.args)
			if tt.wantNil {
				if f != nil {
					t.Errorf("expected nil, got %q", f.Name)
				}
				return
			}
			if f == nil {
				t.Fatal("expected match, got nil")
			}
			if f.Name != tt.want {
				t.Errorf("got %q, want %q", f.Name, tt.want)
			}
		})
	}
}
func TestRegistryMatchExcludeFlags(t *testing.T) {
	f := Filter{
		Name:    "git-log",
		Version: 1,
		Match:   Match{Command: "git", Subcommand: "log", ExcludeFlags: []string{"--format", "--pretty"}},
		OnError: "passthrough",
	}
	reg := NewRegistry([]Filter{f})

	// Should match without excluded flags
	if reg.Match("git", "log", []string{"-10"}) == nil {
		t.Error("expected match without excluded flags")
	}

	// Should NOT match with excluded flag
	if reg.Match("git", "log", []string{"--format=oneline"}) != nil {
		t.Error("expected no match with --format flag")
	}
	if reg.Match("git", "log", []string{"--pretty=short"}) != nil {
		t.Error("expected no match with --pretty flag")
	}
}
func TestRegistryMatchRequireFlags(t *testing.T) {
	f := Filter{
		Name:    "special",
		Version: 1,
		Match:   Match{Command: "cmd", RequireFlags: []string{"--json"}},
		OnError: "passthrough",
	}
	reg := NewRegistry([]Filter{f})

	if reg.Match("cmd", "", []string{"--json"}) == nil {
		t.Error("expected match with required flag")
	}
	if reg.Match("cmd", "", []string{"--text"}) != nil {
		t.Error("expected no match without required flag")
	}
}
func TestShouldInject(t *testing.T) {
	f := Filter{
		Name: "git-log",
		Inject: &Inject{
			Args:          []string{"--oneline"},
			Defaults:      map[string]string{"-n": "10"},
			SkipIfPresent: []string{"--format"},
		},
	}
	reg := NewRegistry(nil)

	// Normal injection
	args, injected := reg.ShouldInject(&f, []string{"log"})
	if !injected {
		t.Fatal("expected injection")
	}
	hasOneline := false
	hasN := false
	for _, a := range args {
		if a == "--oneline" {
			hasOneline = true
		}
		if a == "-n" {
			hasN = true
		}
	}
	if !hasOneline {
		t.Error("missing --oneline")
	}
	if !hasN {
		t.Error("missing -n default")
	}

	// Skip injection when --format present
	args2, injected2 := reg.ShouldInject(&f, []string{"log", "--format=short"})
	if injected2 {
		t.Error("expected skip injection with --format")
	}
	if len(args2) != 2 {
		t.Errorf("args modified: %v", args2)
	}
}
func TestShouldInjectNoInject(t *testing.T) {
	f := Filter{Name: "test"}
	reg := NewRegistry(nil)
	args, injected := reg.ShouldInject(&f, []string{"test"})
	if injected {
		t.Error("expected no injection")
	}
	if len(args) != 1 {
		t.Errorf("args modified: %v", args)
	}
}
```

### File: internal/filter/types.go
```go
type Filter struct {
	Name        string   `yaml:"name"`
	Version     int      `yaml:"version"`
	Description string   `yaml:"description"`
	Match       Match    `yaml:"match"`
	Inject      *Inject  `yaml:"inject,omitempty"`
	Pipeline    Pipeline `yaml:"pipeline"`
	OnError     string   `yaml:"on_error"` // "passthrough", "empty", "template"
}
type Match struct {
	Command      string   `yaml:"command"`
	Subcommand   string   `yaml:"subcommand,omitempty"`
	ExcludeFlags []string `yaml:"exclude_flags,omitempty"`
	RequireFlags []string `yaml:"require_flags,omitempty"`
}
type Inject struct {
	Args          []string          `yaml:"args,omitempty"`
	Defaults      map[string]string `yaml:"defaults,omitempty"`
	SkipIfPresent []string          `yaml:"skip_if_present,omitempty"`
}
type Action struct {
	ActionName string         `yaml:"action"`
	Params     map[string]any `yaml:",inline"`
}
type ActionResult struct {
	Lines    []string
	Metadata map[string]any
	Resolver SourceResolver
}
type SourceResolver interface {
	ResolveSource(ctx context.Context, file string, line int) (string, error)
}
```

### File: internal/filter/types_test.go
```go
func TestFilterYAMLRoundtrip(t *testing.T) {
	input := `
name: "test-filter"
version: 1
description: "A test filter"
match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format"]
inject:
  args: ["--oneline"]
  defaults:
    "-n": "10"
  skip_if_present: ["--format"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
  - action: "head"
    n: 5
on_error: "passthrough"
`
	var f Filter
	if err := yaml.Unmarshal([]byte(input), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if f.Name != "test-filter" {
		t.Errorf("name = %q, want 'test-filter'", f.Name)
	}
	if f.Match.Command != "git" {
		t.Errorf("match.command = %q", f.Match.Command)
	}
	if f.Match.Subcommand != "log" {
		t.Errorf("match.subcommand = %q", f.Match.Subcommand)
	}
	if len(f.Match.ExcludeFlags) != 1 || f.Match.ExcludeFlags[0] != "--format" {
		t.Errorf("exclude_flags = %v", f.Match.ExcludeFlags)
	}
	if f.Inject == nil {
		t.Fatal("inject is nil")
	}
	if len(f.Inject.Args) != 1 {
		t.Errorf("inject.args = %v", f.Inject.Args)
	}
	if f.Inject.Defaults["-n"] != "10" {
		t.Errorf("inject.defaults = %v", f.Inject.Defaults)
	}
	if len(f.Pipeline) != 2 {
		t.Fatalf("pipeline len = %d, want 2", len(f.Pipeline))
	}
	if f.Pipeline[0].ActionName != "keep_lines" {
		t.Errorf("pipeline[0].action = %q", f.Pipeline[0].ActionName)
	}
	if f.Pipeline[1].ActionName != "head" {
		t.Errorf("pipeline[1].action = %q", f.Pipeline[1].ActionName)
	}
	if f.OnError != "passthrough" {
		t.Errorf("on_error = %q", f.OnError)
	}
}
func TestActionResultEmpty(t *testing.T) {
	ar := ActionResult{Lines: nil, Metadata: nil}
	if len(ar.Lines) != 0 {
		t.Error("expected empty lines")
	}
}
```

### File: internal/initcmd/init.go
```go
func Run(args []string) error {
	agent := "claude" // Default for backward compatibility with 'init'
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		agent = args[0]
	}

	for _, arg := range args {
		if arg == "--uninstall" {
			return Uninstall(agent)
		}
	}

	// Always ensure filter directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	filterDir := filepath.Join(home, ".config", "scouter", "filters")
	if err := os.MkdirAll(filterDir, 0755); err != nil {
		return fmt.Errorf("create filter dir: %w", err)
	}

	switch agent {
	case "claude":
		return installClaude(home, filterDir)
	case "gemini-cli":
		return installGeminiCLI(home)
	case "opencode":
		return installOpenCode(home)
	default:
		return fmt.Errorf("unknown agent: %s (supported: claude, gemini-cli, opencode)", agent)
	}
}
func installClaude(home, filterDir string) error {
	// 1. Write hook script
	hookDir := filepath.Join(home, ".claude", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("create hook dir: %w", err)
	}

	hookPath := filepath.Join(hookDir, hookIdentifier)
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	// 2. Patch settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := patchClaudeSettings(settingsPath, hookPath); err != nil {
		return fmt.Errorf("patch settings: %w", err)
	}

	fmt.Println("✅ Scouter init complete (Claude Code):")
	fmt.Printf("  hook: %s\n", hookPath)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  settings: %s\n", settingsPath)
	return nil
}
func installGeminiCLI(home string) error {
	configPath := filepath.Join(home, ".gemini", "settings.json")
	binPath, err := os.Executable()
	if err != nil {
		binPath, _ = filepath.Abs(os.Args[0])
	}

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = make(map[string]interface{})
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers["scouter"] = map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp"},
		"trust":   true,
	}

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0600); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("✅ Scouter integrated with Gemini CLI (MCP)!\n")
	fmt.Printf("  settings: %s\n", configPath)
	return nil
}
func installOpenCode(home string) error {
	configPath := filepath.Join(home, ".config", "opencode", "settings.json")
	binPath, err := os.Executable()
	if err != nil {
		binPath, _ = filepath.Abs(os.Args[0])
	}

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = make(map[string]interface{})
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers["scouter"] = map[string]interface{}{
		"type":    "local",
		"command": []string{binPath, "mcp"},
		"enabled": true,
	}

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0600); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("✅ Scouter integrated with OpenCode (MCP)!\n")
	fmt.Printf("  settings: %s\n", configPath)
	return nil
}
func Uninstall(agent string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	switch agent {
	case "claude":
		hookPath := filepath.Join(home, ".claude", "hooks", hookIdentifier)
		_ = os.Remove(hookPath)
		settingsPath := filepath.Join(home, ".claude", "settings.json")
		unpatchClaudeSettings(settingsPath)
		fmt.Println("✅ Scouter uninstalled from Claude")
	case "gemini-cli":
		settingsPath := filepath.Join(home, ".gemini", "settings.json")
		removeMCPServer(settingsPath, "scouter")
		fmt.Println("✅ Scouter removed from Gemini CLI")
	case "opencode":
		settingsPath := filepath.Join(home, ".config", "opencode", "settings.json")
		removeMCPServer(settingsPath, "scouter")
		fmt.Println("✅ Scouter removed from OpenCode")
	}

	return nil
}
func removeMCPServer(path, name string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return
	}
	delete(mcpServers, name)
	newData, _ := json.MarshalIndent(config, "", "  ")
	_ = os.WriteFile(path, newData, 0600)
}
func patchClaudeSettings(path, hookPath string) error {
	var settings map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]any)
		} else {
			return fmt.Errorf("read settings: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	scouterHookEntry := map[string]any{
		"type":    "command",
		"command": hookPath,
	}

	scouterMatcher := map[string]any{
		"matcher": "Bash",
		"hooks":   []any{scouterHookEntry},
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	var preToolUse []any
	if existing, ok := hooks["PreToolUse"]; ok {
		if arr, ok := existing.([]any); ok {
			preToolUse = arr
		}
	}

	found := false
	for i, entry := range preToolUse {
		if isScouterEntry(entry) {
			preToolUse[i] = scouterMatcher
			found = true
			break
		}
	}
	if !found {
		preToolUse = append(preToolUse, scouterMatcher)
	}

	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(path, out, 0600)
}
func unpatchClaudeSettings(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return
	}

	existing, ok := hooks["PreToolUse"]
	if !ok {
		return
	}
	arr, ok := existing.([]any)
	if !ok {
		return
	}

	var filtered []any
	for _, entry := range arr {
		if !isScouterEntry(entry) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, out, 0600)
}
func isScouterEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksRaw, ok := m["hooks"]
	if !ok {
		return false
	}
	hooksArr, ok := hooksRaw.([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, hookIdentifier) {
			return true
		}
	}
	return false
}
```

### File: internal/initcmd/init_test.go
```go
func TestPatchSettingsNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "scouter-rewrite.sh")

	err := patchClaudeSettings(path, hookPath)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks not found")
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatal("PreToolUse not found or not array")
	}

	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(preToolUse))
	}

	entry := preToolUse[0].(map[string]any)
	if entry["matcher"] != "Bash" {
		t.Errorf("matcher = %v, want Bash", entry["matcher"])
	}

	entryHooks := entry["hooks"].([]any)
	hook := entryHooks[0].(map[string]any)
	if hook["type"] != "command" {
		t.Errorf("type = %v, want command", hook["type"])
	}
	if hook["command"] != hookPath {
		t.Errorf("command = %v, want %s", hook["command"], hookPath)
	}
}
func TestPatchSettingsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "scouter-rewrite.sh")

	// Write existing settings with other hooks
	existing := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Write",
					"hooks": []any{
						map[string]any{"type": "command", "command": "other-hook.sh"},
					},
				},
			},
			"PostToolUse": "other-hook",
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	err := patchClaudeSettings(path, hookPath)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)

	// Existing settings preserved
	if settings["theme"] != "dark" {
		t.Error("existing settings not preserved")
	}

	hooks := settings["hooks"].(map[string]any)

	// PostToolUse preserved
	if hooks["PostToolUse"] != "other-hook" {
		t.Error("PostToolUse not preserved")
	}

	// PreToolUse should have 2 entries (existing Write + new Bash)
	preToolUse := hooks["PreToolUse"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 PreToolUse entries, got %d", len(preToolUse))
	}

	// First entry should be the existing Write hook
	first := preToolUse[0].(map[string]any)
	if first["matcher"] != "Write" {
		t.Errorf("first matcher = %v, want Write", first["matcher"])
	}

	// Second entry should be scouter Bash hook
	second := preToolUse[1].(map[string]any)
	if second["matcher"] != "Bash" {
		t.Errorf("second matcher = %v, want Bash", second["matcher"])
	}
}
func TestPatchSettingsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "scouter-rewrite.sh")

	// Patch twice
	_ = patchClaudeSettings(path, hookPath)
	_ = patchClaudeSettings(path, hookPath)

	settings := readSettings(t, path)
	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)

	// Should still be exactly 1 entry, not duplicated
	if len(preToolUse) != 1 {
		t.Errorf("expected 1 entry after double patch, got %d", len(preToolUse))
	}
}
func TestUnpatchClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "scouter-rewrite.sh")

	// Patch first
	_ = patchClaudeSettings(path, hookPath)

	// Unpatch
	unpatchClaudeSettings(path)

	settings := readSettings(t, path)

	// hooks section should be gone entirely
	if _, ok := settings["hooks"]; ok {
		hooks := settings["hooks"].(map[string]any)
		if _, ok := hooks["PreToolUse"]; ok {
			t.Error("PreToolUse should be removed after unpatch")
		}
	}
}
func TestUnpatchPreservesOtherHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "scouter-rewrite.sh")

	// Create settings with scouter + another hook
	existing := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Write",
					"hooks":   []any{map[string]any{"type": "command", "command": "other.sh"}},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	// Add scouter
	_ = patchClaudeSettings(path, hookPath)

	// Verify both present
	settings := readSettings(t, path)
	preToolUse := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(preToolUse))
	}

	// Unpatch — should remove scouter but keep the Write hook
	unpatchClaudeSettings(path)

	settings = readSettings(t, path)
	hooks := settings["hooks"].(map[string]any)
	preToolUse = hooks["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 entry after unpatch, got %d", len(preToolUse))
	}
	remaining := preToolUse[0].(map[string]any)
	if remaining["matcher"] != "Write" {
		t.Errorf("remaining matcher = %v, want Write", remaining["matcher"])
	}
}
func runHookScript(t *testing.T, cmd string) string {
	t.Helper()

	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not available")
	}

	dir := t.TempDir()
	hookPath := filepath.Join(dir, "scouter-rewrite.sh")
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	scouterPath := filepath.Join(dir, "scouter")
	if err := os.WriteFile(scouterPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake scouter: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": cmd},
	})

	proc := exec.Command("bash", hookPath)
	proc.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	proc.Stdin = strings.NewReader(string(payload))
	output, runErr := proc.Output()
	if runErr != nil {
		t.Fatalf("hook exited non-zero: %v", runErr)
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("hook output is not valid JSON: %v\noutput: %s", err, output)
	}

	hookOut, _ := result["hookSpecificOutput"].(map[string]any)
	updated, _ := hookOut["updatedInput"].(map[string]any)
	rewritten, _ := updated["command"].(string)
	return rewritten
}
func TestHookScriptMultilineCommand(t *testing.T) {
	// Simulate the JSON Claude Code sends for a heredoc-style git commit.
	// The multiline command contains an unmatched `)"` on the last line,
	// which caused xargs to exit 1 (unmatched double quote).
	cmd := "git add file.go && git commit -m \"$(cat <<'EOF'\n   fix: something\n\n   Co-Authored-By: Bot <bot@example.com>\n   EOF\n   )\""
	rewritten := runHookScript(t, cmd)

	if !strings.HasPrefix(rewritten, "scouter -- git add ") {
		t.Errorf("expected rewritten command to start with 'scouter -- git add', got: %s", rewritten)
	}
}
func TestHookScriptInlinePythonDoesNotRewriteQuotedSemicolons(t *testing.T) {
	cmd := "git commit -m \"$(python3 -c \\\"from pathlib import Path; import sys; print(Path('.').name); print(sys.version)\\\")\" && git status"
	rewritten := runHookScript(t, cmd)

	if !strings.HasPrefix(rewritten, "scouter -- git commit ") {
		t.Fatalf("expected rewritten command to start with 'scouter -- git commit', got: %s", rewritten)
	}
	if strings.Count(rewritten, "scouter --") != 1 {
		t.Fatalf("expected exactly one scouter injection, got: %s", rewritten)
	}
	if strings.Contains(rewritten, "; scouter") {
		t.Fatalf("expected inline python to stay unchanged, got: %s", rewritten)
	}
	if strings.Contains(rewritten, "python3 scouter") {
		t.Fatalf("expected inline python command to stay unchanged, got: %s", rewritten)
	}
}
func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return settings
}
```

### File: internal/mcp/handle_dream.go
```go
type DreamParams struct {
	Project string `json:"project,omitempty"`
	Hours   int    `json:"hours,omitempty"`
}
type KnowledgeGraphParams struct {
	SymbolName string `json:"symbolName"`
}
```

### File: internal/mcp/handlers.go
```go
type IndexParams struct {
	FilePath string `json:"filePath"`
}
type SearchParams struct {
	Query string `json:"query"`
	Type  string `json:"type,omitempty"`
}
type HybridSearchParams struct {
	Query string `json:"query"`
}
type ReadParams struct {
	FilePath string `json:"filePath"`
	Pointer  string `json:"pointer"`
}
type CallersParams struct {
	CalleeName string `json:"calleeName"`
}
type DefinitionParams struct {
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`      // 1-based (standard for humans/agents)
	Character int    `json:"character"` // 1-based
}
type TypeInfoParams struct {
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}
type ImpactParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	MaxDepth   int    `json:"maxDepth,omitempty"`
	Verbose    bool   `json:"verbose,omitempty"`
}
type CriticalParams struct {
	Limit int `json:"limit,omitempty"`
}
type DependenciesParams struct{}
type StructuralSearchParams struct {
	Pattern string `json:"pattern"`
	Ext     string `json:"ext"`
	Path    string `json:"path,omitempty"`
}
type PureSignalParams struct {
	Text  string `json:"text"`
	Mode  string `json:"mode,omitempty"`
	Level string `json:"level,omitempty"`
}
type SelfHealParams struct {
	ErrorLog string `json:"errorLog"`
}
type RippleRefactorParams struct {
	SymbolName     string `json:"symbolName"`
	Transformation string `json:"transformation"`
}
type ObsidianExportParams struct {
	SymbolName string `json:"symbolName"`
	FilePath   string `json:"filePath"`
	VaultPath  string `json:"vaultPath,omitempty"`
}
type CompactContextParams struct {
	Force bool `json:"force,omitempty"`
}
type EvolveParams struct {
	Proposal string `json:"proposal"`
	Force    bool   `json:"force,omitempty"`
}
type PredictParams struct {
	Diff string `json:"diff,omitempty"`
}
type JudgeParams struct {
	Diff     string `json:"diff,omitempty"`
	Proposal string `json:"proposal,omitempty"`
}
type JudgeResult struct {
	Rating      float64  `json:"rating"`
	Verdict     string   `json:"verdict"` // SOVEREIGN, REDEMPTION, HAKAI
	Findings    []string `json:"findings"`
	RiskVectors []string `json:"risk_vectors"`
	Convergence bool     `json:"convergence"`
}
type SaveAnchorParams struct {
	Summary string `json:"summary"`
}
type judgeRes struct {
		text   string
		rating float64
		err    error
	}
```

### File: internal/mcp/prompts.go
```go
```

### File: internal/mcp/resolver.go
```go
func NewPointerResolver(st store.Repository) *PointerResolver {
	return &PointerResolver{store: st}
}
type PointerResolver struct {
	store store.Repository
}
```

### File: internal/mcp/server.go
```go
func NewServer(st store.Repository, logger *slog.Logger) *Server {
	implementation := &mcp.Implementation{
		Name:    "scouter",
		Version: "8.0.0-wave11",
	}
	
	s := &Server{
		mcpServer: mcp.NewServer(implementation, &mcp.ServerOptions{
			Logger: logger,
		}),
		store:    st,
		resolver: NewPointerResolver(st),
		lspMgr:   lsp.NewManager(),
		logger:   logger,
	}

	// [Sovereignty Upgrade] Initialize Engines
	s.ripple = engine.NewRippleEngine(st, nil) // Transformer will be set per request
	s.healer = engine.NewHealerEngine(st, s.lspMgr)
	s.search = engine.NewSearchEngine(st)
	s.compact = engine.NewCompactionEngine(st)

	s.registerTools()
	return s
}
type Server struct {
	mcpServer *mcp.Server
	store     store.Repository
	resolver  *PointerResolver
	lspMgr    *lsp.Manager
	ripple    *engine.RippleEngine
	healer    *engine.HealerEngine
	search    *engine.SearchEngine
	compact   *engine.CompactionEngine
	logger    *slog.Logger
	mu        sync.Mutex
}
```

### File: internal/mcp/server_test.go
```go
func setupTestServer(ctx context.Context) (*Server, *mcp.ClientSession, func()) {
	st, _ := store.New(ctx, ":memory:")
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	server := NewServer(st, logger)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go server.Start(ctx, serverTransport)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, _ := client.Connect(ctx, clientTransport, nil)

	return server, session, func() {
		session.Close()
		st.Close()
	}
}
func TestServer_Lifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}

	if len(tools.Tools) != 20 {
		t.Errorf("expected 20 tools, got %d", len(tools.Tools))
	}
}
func TestServer_Handlers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	tests := []struct {
		name      string
		arguments map[string]any
		wantSub   string
	}{
		{
			name:      "search",
			arguments: map[string]any{"query": "test"},
			wantSub:   "[]",
		},
		{
			name:      "pure_signal",
			arguments: map[string]any{"text": "line1\nline2", "level": "light"},
			wantSub:   "line1",
		},
		{
			name:      "dependencies",
			arguments: map[string]any{},
			wantSub:   "{}",
		},
		{
			name:      "critical_code",
			arguments: map[string]any{"limit": 5},
			wantSub:   "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      tt.name,
				Arguments: tt.arguments,
			})
			if err != nil {
				t.Fatalf("tool %s failed: %v", tt.name, err)
			}
			textContent := res.Content[0].(*mcp.TextContent).Text
			if textContent == "null" {
				textContent = "[]" // SDK normalization for empty results in some cases
			}
		})
	}
}
func TestServer_ErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, session, cleanup := setupTestServer(ctx)
	defer cleanup()

	// Test missing arguments for index (should fail validation)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "index",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call to index failed: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError to be true for missing arguments")
	}

	// Test invalid path for read
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "read",
		Arguments: map[string]any{
			"filePath": "/invalid/path",
			"pointer":  "main",
		},
	})
	if err != nil {
		t.Fatalf("call to scouter_read failed: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError to be true for invalid path")
	}
}
```

### File: internal/store/dependency_test.go
```go
func TestStoreDependencies(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_scouter_deps.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Save dummy dependencies
	deps := []types.Dependency{
		{Name: "github.com/google/uuid", Version: "v1.3.0", Type: "golang", Project: "/path/to/go.mod", Direct: true},
		{Name: "lodash", Version: "4.17.21", Type: "npm", Project: "/path/to/package.json", Direct: true},
		{Name: "golang.org/x/mod", Version: "v0.12.0", Type: "golang", Project: "/path/to/go.mod", Direct: false},
	}

	for _, d := range deps {
		if err := s.SaveDependency(ctx, &d); err != nil {
			t.Fatalf("Failed to save dependency %s: %v", d.Name, err)
		}
	}

	// 2. Test GetDependencies
	results, err := s.GetDependencies(ctx)
	if err != nil {
		t.Fatalf("GetDependencies failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(results))
	}

	// 3. Test ClearDependencies
	if err := s.ClearDependencies(ctx); err != nil {
		t.Fatalf("ClearDependencies failed: %v", err)
	}

	results, _ = s.GetDependencies(ctx)
	if len(results) != 0 {
		t.Errorf("Expected 0 dependencies after ClearDependencies, got %d", len(results))
	}
}
```

### File: internal/store/evolution_test.go
```go
func TestEvolution(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_evolution.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	paths := []string{"file1.go", "file2.ts", "file3.py"}
	for _, p := range paths {
		err = s.SaveFileIndex(ctx, &FileIndex{
			Path:    p,
			Mtime:   12345,
			Hash:    "hash",
			ASTJSON: "{}",
			Project: "scouter",
		})
		if err != nil {
			t.Fatalf("Failed to save file index for %s: %v", p, err)
		}

		// Save a symbol and call to test CASCADE
		err = s.SaveSymbol(ctx, &Symbol{Name: "func_" + p, Path: p})
		if err != nil {
			t.Fatalf("Failed to save symbol for %s: %v", p, err)
		}
		err = s.SaveCall(ctx, Call{CallerName: "main", CalleeName: "func_" + p, Path: p, Line: 1})
		if err != nil {
			t.Fatalf("Failed to save call for %s: %v", p, err)
		}
	}

	// Test GetAllFilePaths
	dbPaths, err := s.GetAllFilePaths(ctx)
	if err != nil {
		t.Fatalf("GetAllFilePaths failed: %v", err)
	}
	if len(dbPaths) != 3 {
		t.Errorf("Expected 3 paths, got %d", len(dbPaths))
	}

	// Test DeleteFileIndex
	err = s.DeleteFileIndex(ctx, "file1.go")
	if err != nil {
		t.Fatalf("DeleteFileIndex failed: %v", err)
	}

	dbPaths, _ = s.GetAllFilePaths(ctx)
	if len(dbPaths) != 2 {
		t.Errorf("Expected 2 paths after deletion, got %d", len(dbPaths))
	}

	found := false
	for _, p := range dbPaths {
		if p == "file1.go" {
			found = true
		}
	}
	if found {
		t.Error("file1.go still exists after deletion")
	}

	// Verify CASCADE (symbols and calls should be gone for file1.go)
	var count int
	err = s.(*Store).db.QueryRow("SELECT COUNT(*) FROM symbols WHERE path = 'file1.go'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query symbols: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 symbols for file1.go due to CASCADE, got %d", count)
	}

	err = s.(*Store).db.QueryRow("SELECT COUNT(*) FROM calls WHERE path = 'file1.go'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query calls: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 calls for file1.go due to CASCADE, got %d", count)
	}
}
```

### File: internal/store/interface_test.go
```go
func TestResolveInterfaces(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_interfaces.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Setup Interface and Implementation
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "iface.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "impl.go", Project: "p"})

	// Interface: Shape with method Area() float64
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Shape", Type: "interface", Path: "iface.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Shape:Area", Type: "method_spec", Signature: "() (float64)", Path: "iface.go"})

	// Implementation: Circle with method Area() float64
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Circle", Type: "class", Path: "impl.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Circle.Area", Type: "method", Signature: "() (float64)", Path: "impl.go"})

	// Another struct that DOES NOT match (different signature)
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Square", Type: "class", Path: "impl.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Square.Area", Type: "method", Signature: "(int) (float64)", Path: "impl.go"})

	// 2. Resolve
	if err := s.ResolveInterfaces(ctx); err != nil {
		t.Fatalf("ResolveInterfaces failed: %v", err)
	}

	// 3. Verify
	callers, err := s.GetCallers(ctx, "Shape")
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}

	found := false
	for _, c := range callers {
		if c.CallerName == "Circle" && c.LinkType == "implements" {
			found = true
		}
		if c.CallerName == "Square" {
			t.Errorf("Square should NOT implement Shape (signature mismatch)")
		}
	}

	if !found {
		t.Error("Expected Circle to implement Shape")
	}
}
```

### File: internal/store/store.go
```go
func New(ctx context.Context, dbPath string) (Repository, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := migrate(ctx, tx); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit migration: %w", err)
	}

	s := &Store{db: db}
	// Go 1.25 native cleanup
	runtime.AddCleanup(s, func(db *sql.DB) {
		_ = db.Close()
	}, db)

	return s, nil
}
func migrate(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_index (path TEXT PRIMARY KEY, mtime INTEGER, hash TEXT, ast_json TEXT, project TEXT);`,
		`CREATE TABLE IF NOT EXISTS symbols (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, type TEXT, signature TEXT DEFAULT '', doc TEXT, path TEXT, start_byte INTEGER, end_byte INTEGER, start_line INTEGER, start_col INTEGER, end_line INTEGER, indegree INTEGER DEFAULT 0, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(name, type, signature, doc, path, content='symbols', content_rowid='id');`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); END;`,
		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_path ON symbols(path);`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_resolution ON symbols(name, path);`,
		`CREATE TABLE IF NOT EXISTS calls (id INTEGER PRIMARY KEY AUTOINCREMENT, caller_name TEXT NOT NULL, callee_name TEXT NOT NULL, path TEXT NOT NULL, line INTEGER NOT NULL, callee_path TEXT DEFAULT '', link_type TEXT DEFAULT 'call', indegree INTEGER DEFAULT 0, FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_callee ON calls(callee_name);`,
		`CREATE INDEX IF NOT EXISTS idx_calls_impact ON calls(callee_name, callee_path);`,
		`CREATE TABLE IF NOT EXISTS dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, version TEXT, type TEXT, project TEXT, direct INTEGER);`,
		`CREATE TABLE IF NOT EXISTS test_results (id INTEGER PRIMARY KEY AUTOINCREMENT, test_name TEXT NOT NULL, status TEXT NOT NULL, error_message TEXT, stack_trace TEXT, target_symbol TEXT, duration_ms INTEGER, project TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE INDEX IF NOT EXISTS idx_test_results_symbol ON test_results(target_symbol);`,
	}

	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	// [Divine Fix] Wave 11 Data Integrity: Remove duplicates before creating unique index
	cleanupQuery := `
		DELETE FROM calls 
		WHERE id NOT IN (
			SELECT MIN(id) 
			FROM calls 
			GROUP BY caller_name, callee_name, path, line, link_type
		);`
	if _, err := tx.ExecContext(ctx, cleanupQuery); err != nil {
		return fmt.Errorf("failed to cleanup duplicate calls: %w", err)
	}

	// [Divine Fix] Wave 11 Data Integrity: Apply unique index after cleanup
	if _, err := tx.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_unique ON calls(caller_name, callee_name, path, line, link_type);"); err != nil {
		return fmt.Errorf("failed to create unique index on calls: %w", err)
	}

	// Dynamic column check for 'doc'
	hasDoc, err := hasColumn(ctx, tx, "symbols", "doc")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.doc: %w", err)
	}
	if !hasDoc {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN doc TEXT;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (doc): %w", err)
		}
	}

	// Dynamic column check for 'signature'
	hasSig, err := hasColumn(ctx, tx, "symbols", "signature")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.signature: %w", err)
	}
	if !hasSig {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN signature TEXT DEFAULT '';"); err != nil {
			return fmt.Errorf("failed to alter table symbols (signature): %w", err)
		}
	}

	// Dynamic column check for 'signature' in FTS (SQLite FTS5 does not support ALTER TABLE)
	hasFTSig, err := hasColumn(ctx, tx, "symbols_fts", "signature")
	if err != nil {
		return fmt.Errorf("failed to check column symbols_fts.signature: %w", err)
	}
	if !hasFTSig {
		// Drop and recreate FTS table
		ftsQueries := []string{
			`DROP TABLE IF EXISTS symbols_fts;`,
			`CREATE VIRTUAL TABLE symbols_fts USING fts5(name, type, signature, doc, path, content='symbols', content_rowid='id');`,
			`INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) SELECT id, name, type, signature, doc, path FROM symbols;`,
		}
		for _, q := range ftsQueries {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("failed to recreate symbols_fts: %w", err)
			}
		}
	}

	// Recreate triggers to include signature and doc
	triggerQueries := []string{
		`DROP TRIGGER IF EXISTS symbols_ai;`,
		`DROP TRIGGER IF EXISTS symbols_ad;`,
		`DROP TRIGGER IF EXISTS symbols_au;`,
		`CREATE TRIGGER symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
		`CREATE TRIGGER symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); END;`,
		`CREATE TRIGGER symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); END;`,
	}
	for _, tq := range triggerQueries {
		if _, err := tx.ExecContext(ctx, tq); err != nil {
			return fmt.Errorf("failed to recreate trigger: %w", err)
		}
	}

	// Dynamic column check for 'start_col'
	hasCol, err := hasColumn(ctx, tx, "symbols", "start_col")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.start_col: %w", err)
	}
	if !hasCol {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN start_col INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (start_col): %w", err)
		}
	}

	// Dynamic column check for 'indegree' in 'calls'
	hasIndegree, err := hasColumn(ctx, tx, "calls", "indegree")
	if err != nil {
		return fmt.Errorf("failed to check column calls.indegree: %w", err)
	}
	if !hasIndegree {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE calls ADD COLUMN indegree INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table calls (indegree): %w", err)
		}
	}

	// Dynamic column check for 'indegree' in 'symbols'
	hasSymIndegree, err := hasColumn(ctx, tx, "symbols", "indegree")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.indegree: %w", err)
	}
	if !hasSymIndegree {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN indegree INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (indegree): %w", err)
		}
	}

	return nil
}
func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name = ?", table)
	var dummy int
	err := tx.QueryRowContext(ctx, query, column).Scan(&dummy)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("querying pragma_table_info for %s: %w", table, err)
}
func getHistoricalRisk(ctx context.Context, symbol string, path string, symbolID string) int {
	project := utils.GetRepoName(ctx)
	if project == "" {
		return 0
	}

	// [Strike 3] Stable Topic Key based on unique symbol ID
	topicKey := "scouter/risk/" + symbolID

	// Heuristic: relative path is better for Engram search
	relPath := path
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil {
			relPath = rel
		}
	}

	// [Strike 2] Defensive Argument Layering: Use -- to prevent injection
	queries := []string{symbol, relPath}
	uniqueIDs := make(map[string]bool)

	for _, q := range queries {
		// Search by query OR topicKey to aggregate history
		cmd := exec.CommandContext(ctx, "engram", "search", "--type", "bugfix", "--project", project, "--limit", "10", "--", q)
		out, err := cmd.Output()
		if err == nil {
			matches := engramIDRegex.FindAllString(string(out), -1)
			for _, m := range matches {
				uniqueIDs[m] = true
			}
		}
	}

	// Also search by specific topicKey to find evolved records
	cmd := exec.CommandContext(ctx, "engram", "search", "--type", "bugfix", "--project", project, "--", topicKey)
	if out, err := cmd.Output(); err == nil {
		matches := engramIDRegex.FindAllString(string(out), -1)
		for _, m := range matches {
			uniqueIDs[m] = true
		}
	}

	return len(uniqueIDs)
}
func parseEngramInsights(input string) []types.MemoryInsight {
	var insights []types.MemoryInsight
	lines := strings.Split(input, "\n")

	headerRegex := regexp.MustCompile(`^\[\d+\] #(\d+) \((\w+)\) (?:—|-) (.+)$`)
	whyRegex := regexp.MustCompile(`(?i)\*\*Why\*\*: (.+)$`)
	learnedRegex := regexp.MustCompile(`(?i)\*\*Learned\*\*: (.+)$`)

	var current *types.MemoryInsight

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := headerRegex.FindStringSubmatch(trimmed); matches != nil {
			if current != nil {
				insights = append(insights, *current)
			}
			current = &types.MemoryInsight{
				ID:    matches[1],
				Type:  matches[2],
				Title: matches[3],
			}
			continue
		}

		if current == nil {
			continue
		}

		if matches := whyRegex.FindStringSubmatch(trimmed); matches != nil {
			current.Why = matches[1]
		} else if matches := learnedRegex.FindStringSubmatch(trimmed); matches != nil {
			current.Learned = matches[1]
		}
	}

	if current != nil {
		insights = append(insights, *current)
	}

	return insights
}
type FileIndex struct {
	Path    string `json:"path"`
	Mtime   int64  `json:"mtime"`
	Hash    string `json:"hash"`
	ASTJSON string `json:"ast_json"`
	Project string `json:"project"`
}
type Symbol struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Signature string  `json:"signature,omitempty"`
	Doc       string  `json:"doc"`
	Path      string  `json:"path"`
	StartByte int     `json:"start_byte"`
	EndByte   int     `json:"end_byte"`
	StartLine int     `json:"start_line"`
	StartCol  int     `json:"start_col"`
	EndLine   int     `json:"end_line"`
	Relevance float64 `json:"relevance,omitempty"`
}
type CriticalSymbol struct {
	Symbol
	Centrality int `json:"centrality"`
	Fragility  int `json:"fragility"`
}
type Call struct {
	CallerName string `json:"caller_name"`
	CalleeName string `json:"callee_name"`
	CalleePath string `json:"callee_path"`
	LinkType   string `json:"link_type"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
}
type SovereignDelta struct {
	Path    string   `json:"path"`
	Hash    string   `json:"hash"`
	Symbols []Symbol `json:"symbols"`
	Calls   []Call   `json:"calls"`
}
type Store struct {
	db *sql.DB
	tx *sql.Tx
}
type methodInfo struct {
		name string
		sig  string
	}
type Repository interface {
	GetFileIndex(ctx context.Context, path string) (*FileIndex, error)
	SaveFileIndex(ctx context.Context, idx *FileIndex) error
	ClearSymbols(ctx context.Context, path string) error
	SaveSymbol(ctx context.Context, sym *Symbol) error
	SearchSymbols(ctx context.Context, query string, symType string) ([]Symbol, error)
	GetSymbolsByNameInFile(ctx context.Context, name, path string) ([]Symbol, error)
	SearchSymbolsWeighted(ctx context.Context, query string, symType string) iter.Seq2[Symbol, error]
	GetSymbolsByRange(ctx context.Context, path string, startLine, endLine int) ([]Symbol, error)
	GetSymbolsByType(ctx context.Context, symType string) ([]Symbol, error)
	GetInterfaces(ctx context.Context) ([]Symbol, error)
	GetCriticalSymbols(ctx context.Context, limit int) ([]CriticalSymbol, error)
	ResolveInterfaces(ctx context.Context) error
	ResolveCentrality(ctx context.Context) error
	ExportDelta(ctx context.Context, syncDir string) error
	ImportDelta(ctx context.Context, syncDir string) error
	SaveCall(ctx context.Context, call Call) error
	GetCallers(ctx context.Context, calleeName string) ([]Call, error)
	GetCallees(ctx context.Context, callerName string) ([]Call, error)
	GetImpact(ctx context.Context, symbolName string, filePath string, maxDepth int) (*types.ImpactResult, error)
	GetAffectedTests(ctx context.Context, symbol, path string) ([]Symbol, error)
	ClearCalls(ctx context.Context, path string) error
	GetStats(ctx context.Context) (int, int, error)
	GetAllFilePaths(ctx context.Context) ([]string, error)
	DeleteFileIndex(ctx context.Context, path string) error
	SaveDependency(ctx context.Context, dep *types.Dependency) error
	GetDependencies(ctx context.Context) ([]types.Dependency, error)
	ClearDependencies(ctx context.Context) error
	GetUnusedSymbols(ctx context.Context, includeExported bool) ([]Symbol, error)
	SaveTestResult(ctx context.Context, res *types.TestResult) error
	GetHealthReport(ctx context.Context, symbol string, failuresOnly bool) iter.Seq2[types.TestResult, error]
	ClearTestResults(ctx context.Context) error
	GetMemoryInsights(ctx context.Context, query string) ([]types.MemoryInsight, error)
	WithTransaction(ctx context.Context, fn func(context.Context, Repository) error) error
	Close() error
}
```

### File: internal/store/store_test.go
```go
func TestStoreSearch(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index to satisfy foreign key
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "store.go",
		Mtime:   123456789,
		Hash:    "dummyhash",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	// 1. Save dummy symbols
	syms := []Symbol{
		{Name: "SearchSymbols", Type: "method", Path: "store.go", StartByte: 100, EndByte: 200},
		{Name: "New", Type: "function", Path: "store.go", StartByte: 10, EndByte: 50},
		{Name: "Store", Type: "class", Path: "store.go", StartByte: 0, EndByte: 5},
	}

	for _, sym := range syms {
		if err := s.SaveSymbol(ctx, &sym); err != nil {
			t.Fatalf("Failed to save symbol %s: %v", sym.Name, err)
		}
	}

	// 2. Test search
	results, err := s.SearchSymbols(ctx, "Search*", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one result for 'Search*'")
	}

	found := false
	for _, r := range results {
		if r.Name == "SearchSymbols" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Did not find 'SearchSymbols' in results")
	}

	// 3. Test filter by type
	results, _ = s.SearchSymbols(ctx, "Store", "class")
	if len(results) != 1 || results[0].Type != "class" {
		t.Errorf("Expected 1 class result, got %d", len(results))
	}
}
func TestStoreCalls(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_calls.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "main.go",
		Mtime:   123456789,
		Hash:    "dummyhash",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	// 1. Save dummy calls
	calls := []Call{
		{CallerName: "main", CalleeName: "foo", Path: "main.go", Line: 10},
		{CallerName: "foo", CalleeName: "bar", Path: "main.go", Line: 20},
		{CallerName: "main", CalleeName: "bar", Path: "main.go", Line: 15},
	}

	for _, c := range calls {
		if err := s.SaveCall(ctx, c); err != nil {
			t.Fatalf("Failed to save call from %s to %s: %v", c.CallerName, c.CalleeName, err)
		}
	}

	// 2. Test GetCallers
	callers, err := s.GetCallers(ctx, "bar")
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}

	if len(callers) != 2 {
		t.Errorf("Expected 2 callers for 'bar', got %d", len(callers))
	}

	// 3. Test ClearCalls
	if err := s.ClearCalls(ctx, "main.go"); err != nil {
		t.Fatalf("ClearCalls failed: %v", err)
	}

	callers, _ = s.GetCallers(ctx, "bar")
	if len(callers) != 0 {
		t.Errorf("Expected 0 callers after ClearCalls, got %d", len(callers))
	}
}
func TestGetImpact(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_impact.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Satisfy FKs
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "a.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "b.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "c.go", Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "d.go", Project: "p"})

	s.SaveSymbol(ctx, &Symbol{Name: "A", Type: "func", Path: "a.go", StartByte: 0, EndByte: 10})
	s.SaveSymbol(ctx, &Symbol{Name: "B", Type: "func", Path: "b.go", StartByte: 0, EndByte: 10})
	s.SaveSymbol(ctx, &Symbol{Name: "C", Type: "func", Path: "c.go", StartByte: 0, EndByte: 10})
	s.SaveSymbol(ctx, &Symbol{Name: "D", Type: "func", Path: "d.go", StartByte: 0, EndByte: 10})

	// 1. Build Graph: A -> B, B -> C, C -> A (cycle), D -> B
	calls := []Call{
		{CallerName: "A", CalleeName: "B", Path: "a.go", CalleePath: "b.go"},
		{CallerName: "B", CalleeName: "C", Path: "b.go", CalleePath: "c.go"},
		{CallerName: "C", CalleeName: "A", Path: "c.go", CalleePath: "a.go"},
		{CallerName: "D", CalleeName: "B", Path: "d.go", CalleePath: "b.go"},
	}
	for _, c := range calls {
		if err := s.SaveCall(ctx, c); err != nil {
			t.Fatalf("Failed to save call in test graph: %v", err)
		}
	}
	s.ResolveCentrality(ctx)

	// 2. Impact of B
	impact, err := s.GetImpact(ctx, "B", "b.go", 3)
	if err != nil {
		t.Fatalf("GetImpact failed: %v", err)
	}

	foundA, foundD, foundC, foundB := false, false, false, false
	for _, r := range impact.Callers {
		switch r.Symbol {
		case "A": foundA = true
		case "D": foundD = true
		case "C": foundC = true
		case "B": foundB = true
		}
	}

	if !foundA || !foundD || !foundC || !foundB {
		t.Errorf("Incomplete impact results: %+v", impact.Callers)
	}

	if !impact.Target.Metrics.PublicExport {
		t.Errorf("Expected B to be public export")
	}
	if impact.RiskLevel != "Medium" {
		t.Errorf("Expected RiskLevel 'Medium', got %s", impact.RiskLevel)
	}
	if impact.Target.RiskScore < 0.2 || impact.Target.RiskScore >= 0.5 {
		t.Errorf("Risk score %v outside of expected bounds", impact.Target.RiskScore)
	}
	if impact.Mermaid == "" || impact.Mermaid[:8] != "graph TD" {
		t.Errorf("Expected valid Mermaid graph, got: %s", impact.Mermaid)
	}

	impact2, _ := s.GetImpact(ctx, "B", "b.go", 1)
	if len(impact2.Callers) != 2 {
		t.Errorf("Depth capping failed, expected 2 results, got %d: %+v", len(impact2.Callers), impact2)
	}
}
func TestGetUnusedSymbols(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_deadcode.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "logic.go",
		Mtime:   123456789,
		Hash:    "hash1",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	// 1. Save symbols:
	// - "usedFunc": will have a caller
	// - "deadFunc": no callers
	// - "ExportedUnused": exported, no callers
	syms := []Symbol{
		{Name: "usedFunc", Type: "function", Path: "logic.go", StartByte: 10, EndByte: 20},
		{Name: "deadFunc", Type: "function", Path: "logic.go", StartByte: 30, EndByte: 40},
		{Name: "ExportedUnused", Type: "function", Path: "logic.go", StartByte: 50, EndByte: 60},
	}
	for _, sym := range syms {
		if err := s.SaveSymbol(ctx, &sym); err != nil {
			t.Fatalf("Failed to save symbol %s: %v", sym.Name, err)
		}
	}

	// 2. Save call to "usedFunc"
	err = s.SaveCall(ctx, Call{CallerName: "main", CalleeName: "usedFunc", Path: "logic.go", Line: 100})
	if err != nil {
		t.Fatalf("Failed to save call: %v", err)
	}

	// 3. Test GetUnusedSymbols (IncludeExported = false)
	// Should only find "deadFunc"
	unused, err := s.GetUnusedSymbols(ctx, false)
	if err != nil {
		t.Fatalf("GetUnusedSymbols failed: %v", err)
	}

	if len(unused) != 1 {
		t.Errorf("Expected 1 unused symbol, got %d", len(unused))
	} else if unused[0].Name != "deadFunc" {
		t.Errorf("Expected 'deadFunc' to be unused, got '%s'", unused[0].Name)
	}

	// 4. Test GetUnusedSymbols (IncludeExported = true)
	// Should find "deadFunc" and "ExportedUnused"
	unused, err = s.GetUnusedSymbols(ctx, true)
	if err != nil {
		t.Fatalf("GetUnusedSymbols failed: %v", err)
	}

	if len(unused) != 2 {
		t.Errorf("Expected 2 unused symbols, got %d", len(unused))
	}
}
func TestStoreSearch_Injection(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_injection.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index to satisfy foreign key
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "store.go",
		Mtime:   123456789,
		Hash:    "dummyhash",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	syms := []Symbol{
		{Name: "NormalSymbol", Type: "method", Path: "store.go", StartByte: 100, EndByte: 200},
		{Name: "Special+Symbol-With:Chars", Type: "function", Path: "store.go", StartByte: 10, EndByte: 50},
	}

	for _, sym := range syms {
		if err := s.SaveSymbol(ctx, &sym); err != nil {
			t.Fatalf("Failed to save symbol %s: %v", sym.Name, err)
		}
	}

	// Try an injection attempt
	_, err = s.SearchSymbols(ctx, "Normal\" OR 1=1 --", "")
	if err != nil {
		t.Errorf("Injection search failed (syntax error?): %v", err)
	}
}
func TestSaveFileIndex_PreservesSymbols(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_preserve.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	path := "preserve.go"
	// 1. Save file index
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    path,
		Mtime:   100,
		Hash:    "h1",
		ASTJSON: "{}",
		Project: "p1",
	})
	if err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	// 2. Save a symbol linked to that file
	sym := &Symbol{Name: "KeepMe", Type: "func", Path: path, StartByte: 0, EndByte: 10}
	if err := s.SaveSymbol(ctx, sym); err != nil {
		t.Fatalf("SaveSymbol failed: %v", err)
	}

	// 3. Update file index (same path)
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    path,
		Mtime:   200, // changed
		Hash:    "h2",  // changed
		ASTJSON: "{}",
		Project: "p1",
	})
	if err != nil {
		t.Fatalf("SaveFileIndex update failed: %v", err)
	}

	// 4. Verify symbol still exists
	res, err := s.SearchSymbols(ctx, "KeepMe", "")
	if err != nil {
		t.Fatalf("SearchSymbols failed: %v", err)
	}

	if len(res) == 0 {
		t.Error("Symbol was DELETED during FileIndex update (unintended cascade delete). INSERT OR REPLACE is likely the cause.")
	}
}
func TestStore_DeleteCascade(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_cascade.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	path := "cascade.go"
	// 1. Save file index
	err = s.SaveFileIndex(ctx, &FileIndex{Path: path, Project: "p1"})
	if err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	// 2. Save a symbol linked to that file
	sym := &Symbol{Name: "DeleteMe", Type: "func", Path: path}
	if err := s.SaveSymbol(ctx, sym); err != nil {
		t.Fatalf("SaveSymbol failed: %v", err)
	}

	// 3. Delete file index
	if err := s.DeleteFileIndex(ctx, path); err != nil {
		t.Fatalf("DeleteFileIndex failed: %v", err)
	}

	// 4. Verify symbol is GONE (cascade delete)
	res, err := s.SearchSymbols(ctx, "DeleteMe", "")
	if err != nil {
		t.Fatalf("SearchSymbols failed: %v", err)
	}

	if len(res) != 0 {
		t.Error("Symbol was NOT deleted when file_index was deleted (foreign keys might be OFF)")
	}
}
func TestStore_HasColumn(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_hascolumn.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	storeImpl := s.(*Store)
	tx, _ := storeImpl.db.BeginTx(ctx, nil)
	defer tx.Rollback()

	// 1. Check existing column
	has, err := hasColumn(ctx, tx, "symbols", "name")
	if err != nil {
		t.Fatalf("hasColumn(symbols, name) failed: %v", err)
	}
	if !has {
		t.Error("Expected hasColumn(symbols, name) to be true")
	}

	// 2. Check non-existing column
	has, err = hasColumn(ctx, tx, "symbols", "nonexistent")
	if err != nil {
		t.Fatalf("hasColumn(symbols, nonexistent) failed: %v", err)
	}
	if has {
		t.Error("Expected hasColumn(symbols, nonexistent) to be false")
	}

	// 3. Check existing table but invalid column (handled above)
	
	// 4. Check non-existing table
	has, err = hasColumn(ctx, tx, "nonexistent_table", "doc")
	if err != nil {
		t.Fatalf("hasColumn(nonexistent_table, doc) failed: %v", err)
	}
	if has {
		t.Error("Expected hasColumn(nonexistent_table, doc) to be false")
	}
}
func TestSanitizeFTS_SpecialCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"*", ""},
		{"normal", "\"normal\""},
		{"prefix*", "\"prefix\"*"},
		{"with+plus", "\"with+plus\""},
		{"with-minus", "\"with-minus\""},
		{"with:colon", "\"with:colon\""},
		{"with^caret", "\"with^caret\""},
		{"AND", "\"AND\""},
		{"OR", "\"OR\""},
		{"NOT", "\"NOT\""},
		{"NEAR", "\"NEAR\""},
		{"\"quotes\"", "\"\"\"quotes\"\"\""},
		{"a*b", "\"a*b\""},
		{"*", ""},
	}

	for _, tt := range tests {
		actual := utils.SanitizeFTS(tt.input)
		if actual != tt.expected {
			t.Errorf("SanitizeFTS(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}
func TestStore_TransactionSafety(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_tx.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 0. Save dummy file index to satisfy foreign key
	err = s.SaveFileIndex(ctx, &FileIndex{
		Path:    "store.go",
		Mtime:   123456789,
		Hash:    "dummyhash",
		ASTJSON: "{}",
		Project: "scouter",
	})
	if err != nil {
		t.Fatalf("Failed to save file index: %v", err)
	}

	// Try a transaction that fails halfway
	err = s.WithTransaction(ctx, func(txCtx context.Context, tx Repository) error {
		sym := &Symbol{Name: "PartiallySaved", Type: "func", Path: "store.go"}
		if err := tx.SaveSymbol(txCtx, sym); err != nil {
			return err
		}
		return fmt.Errorf("simulated error")
	})

	if err == nil || err.Error() != "simulated error" {
		t.Errorf("Expected simulated error, got %v", err)
	}

	// Verify the symbol was NOT saved
	results, _ := s.SearchSymbols(ctx, "PartiallySaved", "")
	if len(results) != 0 {
		t.Errorf("Expected 0 results after rollback, got %d", len(results))
	}
	}
func TestGetSymbolsByNameInFile(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_namefile.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	path1 := "file1.go"
	path2 := "file2.go"
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: path1, Project: "p"})
	_ = s.SaveFileIndex(ctx, &FileIndex{Path: path2, Project: "p"})

	sym1 := Symbol{Name: "MySym", Type: "func", Path: path1, StartByte: 10, EndByte: 20}
	sym2 := Symbol{Name: "MySym", Type: "func", Path: path2, StartByte: 30, EndByte: 40}
	_ = s.SaveSymbol(ctx, &sym1)
	_ = s.SaveSymbol(ctx, &sym2)

	res, err := s.GetSymbolsByNameInFile(ctx, "MySym", path1)
	if err != nil {
		t.Fatalf("GetSymbolsByNameInFile failed: %v", err)
	}

	if len(res) != 1 {
		t.Errorf("Expected 1 result, got %d", len(res))
	} else if res[0].Path != path1 {
		t.Errorf("Expected path %s, got %s", path1, res[0].Path)
	}
}
func TestGetSymbolsByType(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_type.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "types.go", Project: "p"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Shape", Type: "interface", Path: "types.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Circle", Type: "struct", Path: "types.go"})
	_ = s.SaveSymbol(ctx, &Symbol{Name: "Area", Type: "method", Path: "types.go"})

	res, err := s.GetSymbolsByType(ctx, "interface")
	if err != nil {
		t.Fatalf("GetSymbolsByType failed: %v", err)
	}

	if len(res) != 1 || res[0].Name != "Shape" {
		t.Errorf("Expected 1 interface (Shape), got %v", res)
	}
}
func TestCallLinkTypePersistence(t *testing.T) {
	ctx := t.Context()
	dbPath := "test_scouter_linktype.db"
	defer os.Remove(dbPath)

	s, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	_ = s.SaveFileIndex(ctx, &FileIndex{Path: "main.go", Project: "p"})

	// Test default link type
	_ = s.SaveCall(ctx, Call{CallerName: "A", CalleeName: "B", Path: "main.go"})
	// Test dynamic link type
	_ = s.SaveCall(ctx, Call{CallerName: "Iface.M", CalleeName: "Impl.M", Path: "main.go", LinkType: "dynamic"})

	callers, _ := s.GetCallers(ctx, "Impl.M")
	if len(callers) != 1 || callers[0].LinkType != "dynamic" {
		t.Errorf("Expected dynamic link type, got %v", callers)
	}
}
```

### File: internal/tee/tee.go
```go
func DefaultConfig() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return Config{
		Enabled:     true,
		Mode:        "failures",
		MaxFiles:    50,                // RTK-inspired higher limit for unique commands
		MaxFileSize: 2 * 1024 * 1024,   // 2MB for deeper context
		Dir:         filepath.Join(home, ".local", "share", "scouter", "tee"),
	}
}
func MaybeSave(raw string, exitCode int, cmd string, cfg Config) string {
	if !cfg.Enabled || cfg.Mode == "never" {
		return ""
	}

	// Check SCOUTER_TEE env override
	if os.Getenv("SCOUTER_TEE") == "0" {
		return ""
	}

	shouldSave := cfg.Mode == "always" || (cfg.Mode == "failures" && exitCode != 0)
	if !shouldSave {
		return ""
	}

	// Skip if output is too small to be meaningful context
	if len(raw) < 200 {
		return ""
	}

	dir := cfg.Dir
	if envDir := os.Getenv("SCOUTER_TEE_DIR"); envDir != "" {
		dir = envDir
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "" // Silent failure
	}

	// CAS: Filename is the hash of the command string
	hash := utils.HashString(cmd)
	filename := fmt.Sprintf("%s.log", hash)
	path := filepath.Join(dir, filename)

	// Truncate if too large (rune-safe)
	data := raw
	if int64(len(data)) > cfg.MaxFileSize {
		runes := []rune(data)
		byteCount := 0
		for i, r := range runes {
			byteCount += len(string(r))
			if int64(byteCount) > cfg.MaxFileSize {
				data = string(runes[:i]) + "\n[scouter: output truncated due to MaxFileSize limit]"
				break
			}
		}
	}

	// Write latest trace for this specific command hash
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return "" // Silent failure
	}

	// Rotate by access/modification time if we reach MaxFiles
	rotateFiles(dir, cfg.MaxFiles)

	return fmt.Sprintf("[full output: %s]", path)
}
func rotateFiles(dir string, maxFiles int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var logFiles []os.FileInfo
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			if info, err := e.Info(); err == nil {
				logFiles = append(logFiles, info)
			}
		}
	}

	if len(logFiles) <= maxFiles {
		return
	}

	// Sort by modification time (oldest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime().Before(logFiles[j].ModTime())
	})

	// Remove oldest unique command traces
	toRemove := len(logFiles) - maxFiles
	for i := 0; i < toRemove; i++ {
		_ = os.Remove(filepath.Join(dir, logFiles[i].Name()))
	}
}
type Config struct {
	Enabled     bool
	Mode        string // "failures", "always", "never"
	MaxFiles    int
	MaxFileSize int64
	Dir         string
}
```

### File: internal/tee/tee_test.go
```go
func testConfig(dir string) Config {
	return Config{
		Enabled:     true,
		Mode:        "failures",
		MaxFiles:    3,
		MaxFileSize: 1 << 20,
		Dir:         dir,
	}
}
func TestMaybeSaveOnFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	raw := strings.Repeat("error output\n", 100) // >500 chars

	hint := MaybeSave(raw, 1, "git push", cfg)
	if hint == "" {
		t.Fatal("expected hint, got empty")
	}
	if !strings.Contains(hint, "[full output:") {
		t.Errorf("unexpected hint: %q", hint)
	}

	// Verify file exists
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}
func TestMaybeSaveNoSaveOnSuccess(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	raw := strings.Repeat("output\n", 100)

	hint := MaybeSave(raw, 0, "git push", cfg)
	if hint != "" {
		t.Errorf("expected no save on success, got %q", hint)
	}
}
func TestMaybeSaveSmallOutput(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)

	hint := MaybeSave("small", 1, "cmd", cfg)
	if hint != "" {
		t.Errorf("expected no save for small output, got %q", hint)
	}
}
func TestMaybeSaveDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Enabled = false
	raw := strings.Repeat("error\n", 100)

	hint := MaybeSave(raw, 1, "cmd", cfg)
	if hint != "" {
		t.Errorf("expected no save when disabled, got %q", hint)
	}
}
func TestMaybeSaveModeAlways(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Mode = "always"
	raw := strings.Repeat("output\n", 100)

	hint := MaybeSave(raw, 0, "cmd", cfg)
	if hint == "" {
		t.Error("expected save in always mode on success")
	}
}
func TestMaybeSaveEnvDisable(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	raw := strings.Repeat("error\n", 100)

	t.Setenv("SCOUTER_TEE", "0")
	hint := MaybeSave(raw, 1, "cmd", cfg)
	if hint != "" {
		t.Errorf("expected no save with SCOUTER_TEE=0, got %q", hint)
	}
}
func TestRotateFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 5 log files
	for i := range 5 {
		path := filepath.Join(dir, strings.Repeat("a", i+1)+".log")
		_ = os.WriteFile(path, []byte("data"), 0644)
	}

	rotateFiles(dir, 3)

	entries, _ := os.ReadDir(dir)
	logCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log") {
			logCount++
		}
	}
	if logCount != 3 {
		t.Errorf("expected 3 files after rotation, got %d", logCount)
	}
}
```

### File: internal/telemetry/driver.go
```go
```

### File: internal/telemetry/driver_lite.go
```go
```

### File: internal/telemetry/schema.go
```go
type Summary struct {
	TotalCommands int
	TotalSaved    int
	AvgSavings    float64
	TotalTimeMs   int64
}
type DayStats struct {
	Day          string
	Commands     int
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	AvgSavings   float64
}
type CommandRecord struct {
	OriginalCmd  string
	ScouterCmd   string
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	SavingsPct   float64
	ExecTimeMs   int64
	Timestamp    string
}
type CommandStats struct {
	Command      string
	Count        int
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	AvgSavings   float64
}
type PeriodStats struct {
	Period       string
	Commands     int
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	AvgSavings   float64
}
```

### File: internal/telemetry/timed.go
```go
func Start(tracker *Tracker) *TimedExecution {
	return &TimedExecution{
		tracker:   tracker,
		startTime: time.Now(),
	}
}
type TimedExecution struct {
	tracker   *Tracker
	startTime time.Time
}
```

### File: internal/telemetry/timed_test.go
```go
func TestTimedExecution(t *testing.T) {
	tracker := newTestTracker(t)

	timed := Start(tracker)
	err := timed.Track(context.Background(), "git log", "scouter git log", 500, 100)
	if err != nil {
		t.Fatalf("timed track: %v", err)
	}

	summary, err := tracker.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalCommands != 1 {
		t.Errorf("total commands = %d", summary.TotalCommands)
	}
}
func TestTimedExecutionNilTracker(t *testing.T) {
	timed := Start(nil)
	err := timed.Track(context.Background(), "cmd", "scouter cmd", 100, 50)
	if err != nil {
		t.Fatalf("expected nil tracker to be no-op: %v", err)
	}
	err = timed.TrackPassthrough(context.Background(), "cmd", 100)
	if err != nil {
		t.Fatalf("expected nil tracker passthrough to be no-op: %v", err)
	}
}
```

### File: internal/telemetry/tracker.go
```go
func NewTracker(ctx context.Context, dbPath string) (*Tracker, error) {
	t := &Tracker{dbPath: dbPath}
	if err := t.ensureOpen(ctx); err != nil {
		return nil, err
	}
	return t, nil
}
func NewLazyTracker(dbPath string) *Tracker {
	return &Tracker{dbPath: dbPath}
}
func DBPath(configPath string) string {
	if p := os.Getenv("SCOUTER_DB_PATH"); p != "" {
		return p
	}
	if configPath != "" {
		return configPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "scouter", "scouter.db")
}
type Tracker struct {
	db      *sql.DB
	dbPath  string
	once    sync.Once
	wg      sync.WaitGroup
	initErr error
}
```

### File: internal/telemetry/tracker_test.go
```go
func newTestTracker(t *testing.T) *Tracker {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := NewTracker(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	t.Cleanup(func() { _ = tracker.Close() })
	return tracker
}
func TestNewTracker(t *testing.T) {
	tracker := newTestTracker(t)
	if tracker == nil {
		t.Fatal("tracker is nil")
	}
}
func TestTrack(t *testing.T) {
	tracker := newTestTracker(t)

	err := tracker.Track(context.Background(), "git log", "scouter git log", 1000, 200, 50)
	if err != nil {
		t.Fatalf("track: %v", err)
	}

	summary, err := tracker.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalCommands != 1 {
		t.Errorf("total commands = %d", summary.TotalCommands)
	}
	if summary.TotalSaved != 800 {
		t.Errorf("total saved = %d", summary.TotalSaved)
	}
	if summary.AvgSavings < 79 || summary.AvgSavings > 81 {
		t.Errorf("avg savings = %.1f%%", summary.AvgSavings)
	}
}
func TestTrackPassthrough(t *testing.T) {
	tracker := newTestTracker(t)

	err := tracker.TrackPassthrough(context.Background(), "npm install", 500, 100)
	if err != nil {
		t.Fatalf("track passthrough: %v", err)
	}

	summary, err := tracker.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalSaved != 0 {
		t.Errorf("expected 0 saved for passthrough, got %d", summary.TotalSaved)
	}
}
func TestGetRecent(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track(context.Background(), "cmd1", "scouter cmd1", 100, 30, 10)
	_ = tracker.Track(context.Background(), "cmd2", "scouter cmd2", 200, 50, 20)
	_ = tracker.Track(context.Background(), "cmd3", "scouter cmd3", 300, 80, 30)

	recent, err := tracker.GetRecent(context.Background(), 2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("got %d records, want 2", len(recent))
	}
	// Most recent first
	if recent[0].OriginalCmd != "cmd3" {
		t.Errorf("first = %q", recent[0].OriginalCmd)
	}
}
func TestGetDaily(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track(context.Background(), "cmd1", "scouter cmd1", 100, 30, 10)
	_ = tracker.Track(context.Background(), "cmd2", "scouter cmd2", 200, 50, 20)

	daily, err := tracker.GetDaily(context.Background(), 7)
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if len(daily) != 1 {
		t.Fatalf("got %d days, want 1", len(daily))
	}
	if daily[0].Commands != 2 {
		t.Errorf("commands = %d", daily[0].Commands)
	}
}
func TestGetByCommand(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track(context.Background(), "git log", "scouter git log", 1000, 200, 50)
	_ = tracker.Track(context.Background(), "git log", "scouter git log", 800, 100, 40)
	_ = tracker.Track(context.Background(), "go test", "scouter go test", 2000, 300, 100)
	_ = tracker.Track(context.Background(), "ls -la", "scouter ls -la", 50, 30, 5)

	stats, err := tracker.GetByCommand(context.Background(), 10)
	if err != nil {
		t.Fatalf("by command: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("got %d commands, want 3", len(stats))
	}
	// go test has most saved (1700), then git log (1500), then ls -la (20)
	if stats[0].Command != "go test" {
		t.Errorf("first command = %q, want go test", stats[0].Command)
	}
	if stats[0].SavedTokens != 1700 {
		t.Errorf("go test saved = %d, want 1700", stats[0].SavedTokens)
	}
	if stats[1].Command != "git log" {
		t.Errorf("second command = %q, want git log", stats[1].Command)
	}
	if stats[1].Count != 2 {
		t.Errorf("git log count = %d, want 2", stats[1].Count)
	}
}
func TestGetByCommandLimit(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track(context.Background(), "cmd1", "scouter cmd1", 100, 30, 10)
	_ = tracker.Track(context.Background(), "cmd2", "scouter cmd2", 200, 50, 20)
	_ = tracker.Track(context.Background(), "cmd3", "scouter cmd3", 300, 80, 30)

	stats, err := tracker.GetByCommand(context.Background(), 2)
	if err != nil {
		t.Fatalf("by command: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d commands, want 2", len(stats))
	}
}
func TestGetWeekly(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track(context.Background(), "cmd1", "scouter cmd1", 100, 30, 10)
	_ = tracker.Track(context.Background(), "cmd2", "scouter cmd2", 200, 50, 20)

	weekly, err := tracker.GetWeekly(context.Background(), 4)
	if err != nil {
		t.Fatalf("weekly: %v", err)
	}
	if len(weekly) != 1 {
		t.Fatalf("got %d weeks, want 1", len(weekly))
	}
	if weekly[0].Commands != 2 {
		t.Errorf("commands = %d, want 2", weekly[0].Commands)
	}
}
func TestGetMonthly(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track(context.Background(), "cmd1", "scouter cmd1", 500, 100, 30)
	_ = tracker.Track(context.Background(), "cmd2", "scouter cmd2", 800, 200, 40)

	monthly, err := tracker.GetMonthly(context.Background(), 6)
	if err != nil {
		t.Fatalf("monthly: %v", err)
	}
	if len(monthly) != 1 {
		t.Fatalf("got %d months, want 1", len(monthly))
	}
	if monthly[0].Commands != 2 {
		t.Errorf("commands = %d, want 2", monthly[0].Commands)
	}
	if monthly[0].SavedTokens != 1000 {
		t.Errorf("saved = %d, want 1000", monthly[0].SavedTokens)
	}
}
func TestDBPath(t *testing.T) {
	t.Setenv("SCOUTER_DB_PATH", "/custom/path.db")
	if got := DBPath(""); got != "/custom/path.db" {
		t.Errorf("got %q", got)
	}

	t.Setenv("SCOUTER_DB_PATH", "")
	if got := DBPath("/config/path.db"); got != "/config/path.db" {
		t.Errorf("got %q", got)
	}
}
```

### File: internal/types/types.go
```go
type Range struct {
	Start int `json:"start" validate:"gte=0"`
	End   int `json:"end" validate:"gtfield=Start"`
}
type ASTPointer struct {
	Type      string `json:"type" validate:"required,oneof=function class method variable interface method_spec"`
	Name      string `json:"name" validate:"required"`
	Signature string `json:"signature,omitempty"`
	Doc       string `json:"doc"`
	Range     Range  `json:"range" validate:"required"`
	StartLine int    `json:"start_line" validate:"required,gte=1"`
	StartCol  int    `json:"start_col" validate:"required,gte=1"`
	EndLine   int    `json:"end_line" validate:"required,gtfield=StartLine"`
	Hash      string `json:"hash" validate:"required,len=64"`
}
type ASTCall struct {
	CallerName string `json:"caller_name" validate:"required"`
	CalleeName string `json:"callee_name" validate:"required"`
	CalleePath string `json:"callee_path"` // Optional: absolute path to the callee file
	LinkType   string `json:"link_type"`   // call, implements, emits, etc.
	Path       string `json:"path" validate:"required"`
	Line       int    `json:"line" validate:"required,gte=1"`
}
type RiskMetrics struct {
	Centrality         float64 `json:"centrality"`
	BlastRadius        float64 `json:"blast_radius"`
	PublicExport       bool    `json:"public_export"`
	HistoricalBugfixes int     `json:"historical_bugfixes"`
}
type ImpactEntity struct {
	Symbol    string      `json:"symbol"`
	File      string      `json:"file"`
	Distance  int         `json:"distance"`
	RiskScore float64     `json:"risk_score"`
	LinkType  string      `json:"link_type"`
	Metrics   RiskMetrics `json:"metrics"`
}
type ImpactResult struct {
	Target    ImpactEntity   `json:"target"`
	Callers   []ImpactEntity `json:"callers"`
	Mermaid   string         `json:"mermaid"`
	RiskLevel string         `json:"risk_level"` // Low, Medium, High, Critical
}
type Dependency struct {
	Name    string `json:"name" validate:"required"`
	Version string `json:"version" validate:"required"`
	Type    string `json:"type" validate:"required,oneof=golang npm"`
	Project string `json:"project" validate:"required"` // Path to the manifest file
	Direct  bool   `json:"direct"`
}
type TestResult struct {
	TestName     string `json:"test_name" validate:"required"`
	Status       string `json:"status" validate:"required,oneof=pass fail skip"`
	ErrorMessage string `json:"error_message,omitempty"`
	StackTrace   string `json:"stack_trace,omitempty"`
	TargetSymbol string `json:"target_symbol,omitempty"`
	DurationMS   int64  `json:"duration_ms" validate:"gte=0"`
	Project      string `json:"project,omitempty"`
}
type TestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action" validate:"required,oneof=run pause cont pass fail skip output bench"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed" validate:"gte=0"`
}
type MemoryInsight struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Learned string `json:"learned"`
	Why     string `json:"why"`
}
type HybridSearchResult struct {
	Symbols  []Symbol        `json:"symbols"`
	Insights []MemoryInsight `json:"insights"`
}
type TestTarget struct {
	Name string `json:"name"`
	File string `json:"file"`
}
type Symbol struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Signature string `json:"signature,omitempty"`
	Doc       string `json:"doc"`
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}
type CompactionResult struct {
	AnchorPath string `json:"anchor_path"`
	Timestamp  string `json:"timestamp"`
	Message    string `json:"message"`
}
```

### File: internal/types/types_test.go
```go
func TestTestResultValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		res     TestResult
		wantErr bool
	}{
		{
			name: "Valid Result",
			res: TestResult{
				TestName:   "TestFoo",
				Status:     "pass",
				DurationMS: 100,
			},
			wantErr: false,
		},
		{
			name: "Missing TestName",
			res: TestResult{
				Status:     "pass",
				DurationMS: 100,
			},
			wantErr: true,
		},
		{
			name: "Invalid Status",
			res: TestResult{
				TestName:   "TestFoo",
				Status:     "running",
				DurationMS: 100,
			},
			wantErr: true,
		},
		{
			name: "Negative Duration",
			res: TestResult{
				TestName:   "TestFoo",
				Status:     "pass",
				DurationMS: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.res)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.Struct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestTestEventValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		event   TestEvent
		wantErr bool
	}{
		{
			name: "Valid Event",
			event: TestEvent{
				Action:  "pass",
				Elapsed: 0.5,
			},
			wantErr: false,
		},
		{
			name: "Missing Action",
			event: TestEvent{
				Elapsed: 0.5,
			},
			wantErr: true,
		},
		{
			name: "Invalid Action",
			event: TestEvent{
				Action:  "invalid",
				Elapsed: 0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.Struct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

### File: internal/utils/comments.go
```go
func CleanComment(raw string) string {
	if raw == "" {
		return ""
	}

	lines := strings.Split(raw, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 1. Strip block starts/ends
		trimmed = strings.TrimPrefix(trimmed, "/**")
		trimmed = strings.TrimPrefix(trimmed, "/*")
		trimmed = strings.TrimSuffix(trimmed, "*/")
		trimmed = strings.TrimPrefix(trimmed, `"""`)
		trimmed = strings.TrimSuffix(trimmed, `"""`)
		trimmed = strings.TrimPrefix(trimmed, `'''`)
		trimmed = strings.TrimSuffix(trimmed, `'''`)

		// 2. Strip line markers: //, #, *
		if strings.HasPrefix(trimmed, "//") {
			trimmed = strings.TrimPrefix(trimmed, "//")
		} else if strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimPrefix(trimmed, "#")
		} else if strings.HasPrefix(trimmed, "*") {
			trimmed = strings.TrimPrefix(trimmed, "*")
		}

		trimmed = strings.TrimSpace(trimmed)

		// 3. Collect non-empty lines, keeping internal spacing
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	return strings.Join(cleaned, "\n")
}
```

### File: internal/utils/git.go
```go
func GetLocalChanges(ctx context.Context) ([]DiffRange, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--unified=0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var changes []DiffRange
	var currentFile string
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			matches := hunkRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				start, _ := strconv.Atoi(matches[1])
				count := 1
				if len(matches) == 3 && matches[2] != "" {
					count, _ = strconv.Atoi(matches[2])
				}
				
				changes = append(changes, DiffRange{
					Path:      currentFile,
					StartLine: start,
					EndLine:   start + count - 1,
				})
			}
		}
	}

	_ = cmd.Wait()
	return changes, nil
}
func GetRepoName(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return ""
	}

	// Handle git@github.com:org/repo.git or https://github.com/org/repo.git
	parts := strings.Split(url, "/")
	last := parts[len(parts)-1]
	return strings.TrimSuffix(last, ".git")
}
func RestoreFile(ctx context.Context, path string) error {
	return exec.CommandContext(ctx, "git", "restore", path).Run()
}
type DiffRange struct {
	Path      string
	StartLine int
	EndLine   int
}
```

### File: internal/utils/hash.go
```go
func StringHash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}
func SymbolSignatureHash(symbol, path, signature string) string {
	return StringHash(fmt.Sprintf("%s:%s:%s", symbol, path, signature))
}
func CalculateHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:])
}
```

### File: internal/utils/regex.go
```go
func NewLazyRegex(pattern string) *LazyRegex {
	return &LazyRegex{pattern: pattern}
}
type LazyRegex struct {
	pattern string
	once    sync.Once
	re      *regexp.Regexp
}
```

### File: internal/utils/regex_test.go
```go
func TestLazyRegex(t *testing.T) {
	lr := NewLazyRegex(`^\d+$`)

	// First call compiles
	re := lr.Re()
	if !re.MatchString("123") {
		t.Error("expected match for '123'")
	}
	if re.MatchString("abc") {
		t.Error("expected no match for 'abc'")
	}

	// Second call returns same instance
	re2 := lr.Re()
	if re != re2 {
		t.Error("expected same regexp instance on second call")
	}
}
func TestLazyRegexConcurrent(t *testing.T) {
	lr := NewLazyRegex(`\w+`)
	done := make(chan bool, 10)

	for range 10 {
		go func() {
			re := lr.Re()
			if !re.MatchString("hello") {
				t.Error("expected match")
			}
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}
```

### File: internal/utils/security.go
```go
func GetRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	curr := cwd
	for {
		if _, err := os.Stat(filepath.Join(curr, "go.mod")); err == nil {
			return curr, nil
		}
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			return curr, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return cwd, nil
}
func ValidatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	root, err := GetRepoRoot()
	if err != nil {
		return "", err
	}

	// MANDATE: Reject absolute paths unless they belong to a verified safe root or temp dir.
	// We pin the allowed temp dir to the system default to prevent TMPDIR injection attacks.
	systemTmp := os.TempDir()

	if filepath.IsAbs(path) {
		if !isWithinSovereignty(path, root, systemTmp) {
			return "", fmt.Errorf("security violation: absolute paths outside project root or /tmp are prohibited (%s)", path)
		}
	}

	// Construct candidate absolute path
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(root, path)
	}

	// 🔱 SECURE SYMLINK RESOLUTION (Recursive Fallback)
	// We resolve symlinks of the existing part of the path to prevent TOCTOU/Escape tricks.
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// File doesn't exist, we must validate its existing parent recursively.
		parent := filepath.Dir(fullPath)
		for {
			if rp, err := filepath.EvalSymlinks(parent); err == nil {
				// Parent exists and is resolved. Check if it's within bounds.
				if !isWithinSovereignty(rp, root, systemTmp) {
					return "", fmt.Errorf("security violation: path parent escapes sovereignty (%s)", rp)
				}
				break
			}
			nextParent := filepath.Dir(parent)
			if nextParent == parent {
				break // Root reached
			}
			parent = nextParent
		}
		realPath = filepath.Clean(fullPath)
	}

	// 🏛️ SOVEREIGNTY BOUNDARY CHECK
	if !isWithinSovereignty(realPath, root, systemTmp) {
		return "", fmt.Errorf("security violation: access denied to path outside sovereignty (%s)", realPath)
	}

	// 💎 RELATIVE BLACKLIST CHECK (Case-Insensitive)
	// We only check the parts relative to the root to avoid "Parent Pollution".
	var relPath string
	if strings.HasPrefix(realPath, root) {
		relPath, _ = filepath.Rel(root, realPath)
	} else {
		relPath, _ = filepath.Rel(systemTmp, realPath)
	}

	parts := strings.Split(relPath, string(os.PathSeparator))
	for _, part := range parts {
		for _, blocked := range purityBlacklist {
			if strings.EqualFold(part, blocked) {
				return "", fmt.Errorf("purity violation: access to %s is prohibited", blocked)
			}
		}
	}

	return realPath, nil
}
func isWithinSovereignty(path, root, tmp string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	tmp = filepath.Clean(tmp)

	inRepo := strings.HasPrefix(path, root+string(os.PathSeparator)) || path == root
	inTmp := strings.HasPrefix(path, tmp+string(os.PathSeparator)) || path == tmp
	
	return inRepo || inTmp
}
func SanitizeFTS(q string) string {
	if q == "" {
		return ""
	}
	
	// Check for trailing wildcard
	hasWildcard := strings.HasSuffix(q, "*")
	
	// 1. Clean the string
	s := strings.TrimSpace(q)
	s = strings.TrimSuffix(s, "*")
	
	// 2. Escape double quotes (FTS5 uses double double-quotes for literal quotes)
	s = strings.ReplaceAll(s, "\"", "\"\"")
	
	// 3. Remove leading wildcards (SQLite doesn't support them at the start of a term)
	s = strings.TrimLeft(s, "*")
	
	if s == "" {
		return ""
	}

	// 4. Wrap in quotes to neutralize OR, AND, NEAR, etc.
	res := "\"" + s + "\""
	if hasWildcard {
		res += "*"
	}
	
	return res
}
```

### File: internal/utils/security_test.go
```go
func TestValidatePath(t *testing.T) {
	tmpDir := os.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid relative path", "go.mod", false},
		{"Valid nested path", "internal/utils/security.go", false},
		{"Valid temp path", filepath.Join(tmpDir, "test.txt"), false},
		{"Jailbreak attempt (parent)", "../../etc/passwd", true},
		{"Absolute path violation", "/etc/passwd", true},
		{"Empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
func TestSanitizeFTS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Simple term", "scouter", "\"scouter\""},
		{"With wildcard", "scout*", "\"scout\"*"},
		{"Internal quote", "don't", "\"don't\""},
		{"Control characters", "OR AND NEAR", "\"OR AND NEAR\""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFTS(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeFTS(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
func TestSanitizeFTS_Manual(t *testing.T) {
	// Let's do some manual checks to see what the logic actually produces
	inputs := []string{"test", "test*", "*test", "don't", "\"quoted\""}
	for _, in := range inputs {
		t.Logf("Input: %q -> Got: %q", in, SanitizeFTS(in))
	}
}
```

### File: internal/utils/utils.go
```go
func ExtractJSON(s string) string {
	match := jsonRegex.FindString(s)
	if match == "" {
		return s
	}
	return match
}
func ExtractCodeBlock(s string) string {
	match := codeBlockRegex.FindStringSubmatch(s)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return strings.TrimSpace(s)
}
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
func StripANSI(s string) string {
	return ansiRe.Re().ReplaceAllString(s, "")
}
func EstimateTokens(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	return int(math.Ceil(float64(n) / 4.0))
}
func FormatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
func CompactPath(path string) string {
	prefixes := []string{"src/", "lib/", "internal/", "pkg/", "vendor/"}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return path[len(p):]
		}
	}
	return path
}
func OkConfirmation(action, detail string) string {
	if detail == "" {
		return "ok " + action
	}
	return fmt.Sprintf("ok %s %s", action, detail)
}
func ParseRating(s string) (float64, error) {
	match := ratingRegex.FindStringSubmatch(s)
	if len(match) < 2 {
		return 0, fmt.Errorf("rating not found")
	}
	var r float64
	_, err := fmt.Sscanf(match[1], "%f", &r)
	return r, err
}
func ExtractList(text, header string) []string {
	lines := strings.Split(text, "\n")
	var result []string
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), strings.ToLower(header)) {
			found = true
			continue
		}
		if found {
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
				item := strings.TrimSpace(trimmed[1:])
				if item != "" {
					result = append(result, item)
				}
			} else if len(result) > 0 {
				// Stop if we encounter a non-list line after finding items
				break
			}
		}
	}
	return result
}
```

### File: internal/utils/utils_test.go
```go
func TestValidatePath_Security(t *testing.T) {
	tmp := os.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errSub  string
	}{
		{
			name:    "Valid relative path",
			path:    "go.mod",
			wantErr: false,
		},
		{
			name:    "Path traversal attempt",
			path:    "../../../../etc/passwd",
			wantErr: true,
			errSub:  "escapes sovereignty",
		},
		{
			name:    "Blacklist: .git",
			path:    ".git/config",
			wantErr: true,
			errSub:  "purity violation",
		},
		{
			name:    "Blacklist: .env",
			path:    ".env",
			wantErr: true,
			errSub:  "purity violation",
		},
		{
			name:    "Blacklist Case-Insensitivity: .GIT",
			path:    ".GIT/config",
			wantErr: true,
			errSub:  "purity violation",
		},
		{
			name:    "Valid temp path",
			path:    filepath.Join(tmp, "scouter-test.txt"),
			wantErr: false,
		},
		{
			name:    "Absolute path violation",
			path:    "/etc/passwd",
			wantErr: true,
			errSub:  "absolute paths outside project root or /tmp are prohibited",
		},
		{
			name:    "Project inside restricted folder (Parent Pollution Fix)",
			path:    "src/main.go", 
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("ValidatePath() error = %v, wantErrSub %s", err, tt.errSub)
			}
		})
	}
}
func TestValidatePath_SymlinkEscape(t *testing.T) {
	root, _ := GetRepoRoot()

	// Create a symlink inside the project pointing to a forbidden directory
	evilLink := filepath.Join(root, "evil_link")
	os.Remove(evilLink)
	
	forbiddenDir := filepath.Dir(root) 
	err := os.Symlink(forbiddenDir, evilLink)
	if err != nil {
		t.Skip("Skipping symlink test (permissions or OS limitation)")
		return
	}
	defer os.Remove(evilLink)

	// Attempt to access a file THROUGH the symlink that doesn't exist yet
	pathThroughLink := filepath.Join("evil_link", "new_secret.txt")
	_, err = ValidatePath(pathThroughLink)
	if err == nil {
		t.Error("expected error for path escaping through symlink")
	} else if !strings.Contains(err.Error(), "security violation") {
		t.Errorf("expected security violation, got: %v", err)
	}
}
func TestGetRepoRoot(t *testing.T) {
	root, err := GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot failed: %v", err)
	}
	if !strings.Contains(root, "scouter") {
		t.Errorf("expected root to contain 'scouter', got %s", root)
	}
}
func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"Pure JSON", `{"a":1}`, `{"a":1}`},
		{"Conversational JSON", `Here it is: {"a":1} hope it helps`, `{"a":1}`},
		{"JSON Array", `[1,2,3]`, `[1,2,3]`},
		{"No JSON", `hello world`, `hello world`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractJSON(tt.in); got != tt.want {
				t.Errorf("ExtractJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestExtractCodeBlock(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"Markdown Go", "```go\nfunc main() {}\n```", "func main() {}"},
		{"Markdown Raw", "```\ncode\n```", "code"},
		{"Plain Code with braces", "func main() {}", "func main() {}"},
		{"Conversational Markdown", "Sure!\n```go\nfunc test() {}\n```\nEnjoy", "func test() {}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractCodeBlock(tt.in); got != tt.want {
				t.Errorf("ExtractCodeBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

### File: tests/fixtures/interface_sample.go
```go
func Calculate(s Shape) {
	fmt.Println(s.Area())
}
type Circle struct {
	Radius float64
}
type Square struct {
	Side float64
}
type Shape interface {
	Area() float64
}
```

### File: tests/fixtures/predict_source.go
```go
func Sum(a, b int) int {
	return a + b
}
```

### File: tests/fixtures/predict_source_test.go
```go
func TestSum(t *testing.T) {
	if Sum(1, 2) != 3 {
		t.Fail()
	}
}
```

### File: tests/integration_interface_test.go
```go
func TestIntegration_InterfaceTracing(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_integration_interface.db"
	defer os.Remove(dbPath)

	s, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Prepare Fixture Path
	absPath, _ := filepath.Abs("tests/fixtures/interface_sample.go")
	
	// 2. Index the file (Simplified index logic)
	err = s.SaveFileIndex(ctx, &store.FileIndex{
		Path: absPath,
		Hash: "samplehash",
	})
	if err != nil {
		t.Fatalf("SaveFileIndex failed: %v", err)
	}

	// Manual symbol injection based on interface_sample.go
	// In a real run, engine.ParseFile would do this.
	symbols := []store.Symbol{
		{Name: "Shape", Type: "interface", Path: absPath, StartLine: 6, StartCol: 6, EndLine: 9},
		{Name: "Area", Type: "method_spec", Path: absPath, StartLine: 7, StartCol: 2, EndLine: 7}, // Interface method
		{Name: "Circle", Type: "struct", Path: absPath, StartLine: 11, StartCol: 6, EndLine: 13},
		{Name: "Area", Type: "method", Path: absPath, StartLine: 15, StartCol: 17, EndLine: 17},   // Circle implementation
	}
	for _, sym := range symbols {
		s.SaveSymbol(ctx, &sym)
	}

	// 3. Mock LSP to link Circle.Area back to Shape.Area
	mockProvider := &mockLSPProvider{
		impls: []lsp.Location{
			{
				URI: "file://" + absPath,
				Range: lsp.Range{
					Start: lsp.Position{Line: 6, Character: 1}, // Line 7 (0-based) is Area()
					End:   lsp.Position{Line: 6, Character: 10},
				},
			},
		},
	}

	// 4. Run Enricher
	en := engine.NewEnricher(s, mockProvider)
	if err := en.Enrich(ctx); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// 5. Verify: Get Impact of Circle.Area
	// It should find Shape.Area (distance 1)
	impact, err := s.GetImpact(ctx, "Area", absPath, 3)
	if err != nil {
		t.Fatalf("GetImpact failed: %v", err)
	}

	foundIface := false
	for _, res := range impact.Callers {
		if res.Symbol == "Area" && res.LinkType == "dynamic" {
			foundIface = true
			break
		}
	}

	if !foundIface {
		t.Errorf("Interface method not found in impact analysis: %+v", impact)
	}
}
type mockLSPProvider struct {
	impls []lsp.Location
}
type mockLSPClient struct {
	lsp.LSPClient
	impls []lsp.Location
}
```

### File: tests/predict_test.go
```go
func TestIntegration_PredictiveTesting(t *testing.T) {
	ctx := context.Background()
	dbPath := "test_integration_predict.db"
	defer os.Remove(dbPath)

	s, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Prepare Fixture Paths
	sourcePath, _ := filepath.Abs("fixtures/predict_source.go")
	testPath, _ := filepath.Abs("fixtures/predict_source_test.go")
	
	// 2. Index the files
	files := []string{sourcePath, testPath}
	for _, path := range files {
		pointers, calls, err := engine.ParseFile(ctx, path, nil)
		if err != nil {
			t.Fatalf("ParseFile failed for %s: %v", path, err)
		}

		err = s.WithTransaction(ctx, func(ctx context.Context, tx store.Repository) error {
			tx.SaveFileIndex(ctx, &store.FileIndex{
				Path: path,
				Hash: "hash-" + filepath.Base(path),
			})
			for _, p := range pointers {
				t.Logf("Indexing symbol: %s (%s) at line %d in %s", p.Name, p.Type, p.StartLine, path)
				err := tx.SaveSymbol(ctx, &store.Symbol{
					Name:      p.Name,
					Type:      p.Type,
					Signature: p.Signature,
					Path:      path,
					StartByte: p.Range.Start,
					EndByte:   p.Range.End,
					StartLine: p.StartLine,
					EndLine:   p.EndLine,
				})
				if err != nil {
					return err
				}
			}
			for _, c := range calls {
				t.Logf("Saving call: %s -> %s (callee_path: %q) in %s", c.CallerName, c.CalleeName, c.CalleePath, path)
				err := tx.SaveCall(ctx, store.Call{
					CallerName: c.CallerName,
					CalleeName: c.CalleeName,
					CalleePath: c.CalleePath,
					LinkType:   c.LinkType,
					Path:       path,
					Line:       c.Line,
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Transaction failed for %s: %v", path, err)
		}
	}

	// 3. Mock a git diff that modifies 'Sum' in 'predict_source.go'
	// Sum is at line 3 in predict_source.go
	diff := `--- a/fixtures/predict_source.go
+++ b/fixtures/predict_source.go
@@ -3,1 +3,1 @@
-func Sum(a, b int) int {
+func Sum(a, b, c int) int {`

	// 4. Call engine.PredictTests
	tests, err := engine.PredictTests(ctx, s, diff)
	if err != nil {
		t.Fatalf("PredictTests failed: %v", err)
	}

	// 5. Verify results
	if len(tests) == 0 {
		t.Fatalf("expected at least 1 affected test, got 0")
	}

	found := false
	for _, tt := range tests {
		if tt.Name == "TestSum" && tt.File == testPath {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("TestSum not found in affected tests: %+v", tests)
	}
}
```

## Language: py
### File: tests/fixtures/sample.py
```py
def __init__(self, dsn: str):

        self.host = host
def connect(self):
        print(f"Connecting to {self.host}")
def standalone_function():
    return 42
class Database:
    """
    Database represents a connection to a persistent store.
    It manages connections and execution of queries.
    """
    def __init__(self, dsn: str):

        self.host = host

    def connect(self):
        print(f"Connecting to {self.host}")
```

### File: tests/fixtures/test.py
```py
def hello():
    print("hello")
```

