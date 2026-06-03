import re

with open("internal/engine/truth.go", "r") as f:
    content = f.read()

# 1. Add import "iter"
content = re.sub(
    r'import \(\n',
    'import (\n\t"context"\n\t"fmt"\n\t"iter"\n',
    content,
    count=1
)
content = re.sub(r'\t"context"\n\t"fmt"\n', '', content, count=1)

# 2. Modify TruthEngine.Index
old_index = """	collector := newIndexCollector(ctx, e.store)

	// Limitamos a 4 workers para evitar que el OS (ej. MIUI) mate el proceso por exceso de hilos
	maxWorkers := 4
	if runtime.NumCPU() < 4 {
		maxWorkers = runtime.NumCPU()
	}
	workerSem := make(chan struct{}, maxWorkers)

	var indexErr error
	if fi.IsDir() {
		_, indexErr = e.indexDirectory(ctx, validatedPath, workerSem, collector)
	} else {
		workerSem <- struct{}{}
		_, indexErr = e.indexFile(ctx, validatedPath, workerSem, collector)
		<-workerSem
	}"""

new_index = """	var parsedData map[string]*ParsedPackageData
	if fi.IsDir() {
		pd, loadErr := BatchLoadPackages(validatedPath)
		if loadErr != nil {
			e.logger.Warn("BatchLoadPackages failed, falling back to individual loading", "error", loadErr)
			parsedData = make(map[string]*ParsedPackageData)
		} else {
			parsedData = pd
		}
	} else {
		parsedData = make(map[string]*ParsedPackageData)
	}

	collector := newIndexCollector(ctx, e.store)

	// Limitamos a 4 workers para evitar que el OS (ej. MIUI) mate el proceso por exceso de hilos
	maxWorkers := 4
	if runtime.NumCPU() < 4 {
		maxWorkers = runtime.NumCPU()
	}
	workerSem := make(chan struct{}, maxWorkers)

	var indexErr error
	if fi.IsDir() {
		_, indexErr = e.indexDirectory(ctx, validatedPath, workerSem, collector, parsedData)
	} else {
		workerSem <- struct{}{}
		_, indexErr = e.indexFile(ctx, validatedPath, workerSem, collector, parsedData)
		<-workerSem
	}"""

content = content.replace(old_index, new_index)

# 3. Modify indexDirectory
content = content.replace(
    'func (e *TruthEngine) indexDirectory(ctx context.Context, dir string, workerSem chan struct{}, collector *indexCollector) (string, error) {',
    'func (e *TruthEngine) indexDirectory(ctx context.Context, dir string, workerSem chan struct{}, collector *indexCollector, parsedData map[string]*ParsedPackageData) (string, error) {'
)

content = content.replace(
    'childHash, err := e.indexDirectory(ctx, path, workerSem, collector)',
    'childHash, err := e.indexDirectory(ctx, path, workerSem, collector, parsedData)'
)

content = content.replace(
    'childHash, err := e.indexFile(ctx, path, workerSem, collector)',
    'childHash, err := e.indexFile(ctx, path, workerSem, collector, parsedData)'
)

# 4. Modify indexFile
content = content.replace(
    'func (e *TruthEngine) indexFile(ctx context.Context, path string, workerSem chan struct{}, collector *indexCollector) (string, error) {',
    'func (e *TruthEngine) indexFile(ctx context.Context, path string, workerSem chan struct{}, collector *indexCollector, parsedData map[string]*ParsedPackageData) (string, error) {'
)

old_stream = """	e.logger.Info("indexing file", "path", path)

	itPointers, itCalls, itFlows, err := StreamSymbols(ctx, path)
	if err != nil {
		return "", fmt.Errorf("parsing failed for %s: %w", path, err)
	}"""

new_stream = """	e.logger.Info("indexing file", "path", path)

	var itPointers iter.Seq[types.ASTPointer]
	var itCalls iter.Seq[types.ASTCall]
	var itFlows iter.Seq[types.DataFlow]
	var streamErr error

	if parsedData != nil && filepath.Ext(path) == ".go" {
		if pd, ok := parsedData[path]; ok {
			itPointers, itCalls, itFlows, streamErr = StreamSymbolsFromAST(ctx, pd.Fset, pd.File, pd.Pkg)
		} else {
			itPointers, itCalls, itFlows, streamErr = StreamSymbols(ctx, path)
		}
	} else {
		itPointers, itCalls, itFlows, streamErr = StreamSymbols(ctx, path)
	}

	if streamErr != nil {
		return "", fmt.Errorf("parsing failed for %s: %w", path, streamErr)
	}"""

content = content.replace(old_stream, new_stream)

with open("internal/engine/truth.go", "w") as f:
    f.write(content)

print("Patched truth.go")
