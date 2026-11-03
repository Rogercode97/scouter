package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

// TestCompareImpactCallers pins the semantics of the precision arithmetic before
// anything uses it: what counts as a true positive, a false positive and a false
// negative, and what happens to a known false positive that stops appearing.
func TestCompareImpactCallers(t *testing.T) {
	tests := []struct {
		name           string
		expected       []string
		got            []string
		knownFP        []string
		knownFN        []string
		wantTP         []string
		wantFP         []string
		wantFN         []string
		wantStale      []string
		wantStaleMiss  []string
		wantAbsorbedFP []string
		wantAbsorbedFN []string
	}{
		{
			name:     "exact match leaves nothing to report",
			expected: []string{"a", "b"},
			got:      []string{"a", "b"},
			wantTP:   []string{"a", "b"},
		},
		{
			name:     "an extra caller is a false positive",
			expected: []string{"a"},
			got:      []string{"a", "b"},
			wantTP:   []string{"a"},
			wantFP:   []string{"b"},
		},
		{
			name:     "a missing caller is a false negative",
			expected: []string{"a", "b"},
			got:      []string{"a"},
			wantTP:   []string{"a"},
			wantFN:   []string{"b"},
		},
		{
			name:     "an empty answer against an empty truth is exact",
			expected: nil,
			got:      nil,
		},
		{
			name:           "a known false positive is absorbed, not reported",
			expected:       []string{"a"},
			got:            []string{"a", "b"},
			knownFP:        []string{"b"},
			wantTP:         []string{"a"},
			wantAbsorbedFP: []string{"b"},
		},
		{
			name:      "a known false positive that no longer appears is stale",
			expected:  []string{"a"},
			got:       []string{"a"},
			knownFP:   []string{"b"},
			wantTP:    []string{"a"},
			wantStale: []string{"b"},
		},
		{
			name:           "a known false negative is absorbed, not reported",
			expected:       []string{"a", "b"},
			got:            []string{"a"},
			knownFN:        []string{"b"},
			wantTP:         []string{"a"},
			wantAbsorbedFN: []string{"b"},
		},
		{
			name:          "a known false negative that now appears is stale",
			expected:      []string{"a", "b"},
			got:           []string{"a", "b"},
			knownFN:       []string{"b"},
			wantTP:        []string{"a", "b"},
			wantStaleMiss: []string{"b"},
		},
		{
			name:     "duplicates in the answer are not double counted",
			expected: []string{"a"},
			got:      []string{"a", "a", "a"},
			wantTP:   []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareImpactCallers(tt.expected, tt.got, tt.knownFP, tt.knownFN)
			assertSet(t, "true positives", tt.wantTP, got.TruePositives)
			assertSet(t, "false positives", tt.wantFP, got.FalsePositives)
			assertSet(t, "false negatives", tt.wantFN, got.FalseNegatives)
			assertSet(t, "stale known false positives", tt.wantStale, got.StaleKnownFalsePositives)
			assertSet(t, "stale known false negatives", tt.wantStaleMiss, got.StaleKnownFalseNegatives)
			assertSet(t, "absorbed false positives", tt.wantAbsorbedFP, got.AbsorbedFalsePositives)
			assertSet(t, "absorbed false negatives", tt.wantAbsorbedFN, got.AbsorbedFalseNegatives)
		})
	}
}

func assertSet(t *testing.T, label string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: want %v, got %v", label, want, got)
	}
	seen := make(map[string]bool, len(want))
	for _, w := range want {
		seen[w] = true
	}
	for _, g := range got {
		if !seen[g] {
			t.Fatalf("%s: want %v, got %v", label, want, got)
		}
	}
}

// impactComparison is the arithmetic result of one measured symbol.
type impactComparison struct {
	TruePositives            []string
	FalsePositives           []string
	FalseNegatives           []string
	StaleKnownFalsePositives []string
	StaleKnownFalseNegatives []string
	AbsorbedFalsePositives   []string
	AbsorbedFalseNegatives   []string
}

// compareImpactCallers diffs a measured answer against hand-labelled truth.
// A caller already recorded as a known false positive is absorbed instead of
// reported, and disappears from the ratchet once it stops appearing, so the
// ratchet cannot quietly outlive the defect it documents.
func compareImpactCallers(expected, got, knownFalsePositives, knownFalseNegatives []string) impactComparison {
	expectedSet := setOf(expected)
	gotSet := setOf(got)
	knownSet := setOf(knownFalsePositives)
	knownMissSet := setOf(knownFalseNegatives)

	var result impactComparison
	for name := range expectedSet {
		if gotSet[name] {
			result.TruePositives = append(result.TruePositives, name)
			continue
		}
		if knownMissSet[name] {
			result.AbsorbedFalseNegatives = append(result.AbsorbedFalseNegatives, name)
			continue
		}
		result.FalseNegatives = append(result.FalseNegatives, name)
	}
	for name := range gotSet {
		if expectedSet[name] {
			continue
		}
		if knownSet[name] {
			result.AbsorbedFalsePositives = append(result.AbsorbedFalsePositives, name)
			continue
		}
		result.FalsePositives = append(result.FalsePositives, name)
	}
	for name := range knownSet {
		if !gotSet[name] {
			result.StaleKnownFalsePositives = append(result.StaleKnownFalsePositives, name)
		}
	}
	for name := range knownMissSet {
		if gotSet[name] {
			result.StaleKnownFalseNegatives = append(result.StaleKnownFalseNegatives, name)
		}
	}

	result.TruePositives = sortedCopy(result.TruePositives)
	result.FalsePositives = sortedCopy(result.FalsePositives)
	result.FalseNegatives = sortedCopy(result.FalseNegatives)
	result.StaleKnownFalsePositives = sortedCopy(result.StaleKnownFalsePositives)
	result.StaleKnownFalseNegatives = sortedCopy(result.StaleKnownFalseNegatives)
	result.AbsorbedFalsePositives = sortedCopy(result.AbsorbedFalsePositives)
	result.AbsorbedFalseNegatives = sortedCopy(result.AbsorbedFalseNegatives)
	return result
}

func setOf(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func sortedCopy(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

// impactTruth is the hand-labelled answer key for one corpus.
type impactTruth struct {
	Language string        `json:"language"`
	Root     string        `json:"root"`
	Symbols  []impactEntry `json:"symbols"`
}

type impactEntry struct {
	Query               string   `json:"query"`
	Why                 string   `json:"why"`
	ExpectedCallers     []string `json:"expected_callers"`
	KnownFalsePositives []string `json:"known_false_positives"`
	KnownFalseNegatives []string `json:"known_false_negatives"`
	ExpectedError       bool     `json:"expected_error"`
}

// TestImpactPrecision_Python measures `scouter impact` against truth read from
// the fixture sources. It runs the built binary, so the measurement covers the
// name resolution, the index and the rendering, not just the engine.
func TestImpactPrecision_Python(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	corpusRoot := filepath.Join(wd, "fixtures", "impact_truth", "python")

	sc := buildScouterForImpact(t, repoRoot)
	bin, configPath := sc.bin, sc.configPath

	if out, err := runScouter(t, bin, configPath, "index", corpusRoot); err != nil {
		t.Fatalf("index failed: %v\n%s", err, out)
	}

	raw, err := os.ReadFile(filepath.Join(corpusRoot, "truth.json"))
	if err != nil {
		t.Fatalf("read truth: %v", err)
	}
	var truth impactTruth
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatalf("parse truth: %v", err)
	}
	if len(truth.Symbols) == 0 {
		t.Fatal("truth file has no symbols")
	}

	totalTP, totalFP, totalFN := 0, 0, 0
	for _, entry := range truth.Symbols {
		t.Run(entry.Query, func(t *testing.T) {
			out, runErr := runScouter(t, bin, configPath, "impact", entry.Query, corpusRoot)

			if entry.ExpectedError {
				if runErr == nil {
					t.Fatalf("%q is ambiguous in the corpus but impact answered instead of refusing:\n%s", entry.Query, out)
				}
				if !strings.Contains(strings.ToLower(out), "ambiguous") {
					t.Fatalf("%q failed, but not by refusing the ambiguity:\n%s", entry.Query, out)
				}
				t.Logf("%-10s refused as expected", entry.Query)
				return
			}

			if runErr != nil {
				t.Fatalf("%q: impact failed: %v\n%s", entry.Query, runErr, out)
			}
			callers := parseImpactCallers(out)
			cmp := compareImpactCallers(entry.ExpectedCallers, callers, entry.KnownFalsePositives, entry.KnownFalseNegatives)
			t.Logf("%-10s callers=%v tp=%d fp=%v fn=%v known_fp=%v known_fn=%v",
				entry.Query, callers, len(cmp.TruePositives), cmp.FalsePositives, cmp.FalseNegatives, cmp.AbsorbedFalsePositives, cmp.AbsorbedFalseNegatives)

			totalTP += len(cmp.TruePositives)
			totalFP += len(cmp.FalsePositives) + len(cmp.AbsorbedFalsePositives)
			totalFN += len(cmp.FalseNegatives) + len(cmp.AbsorbedFalseNegatives)

			if len(cmp.FalseNegatives) != 0 {
				t.Errorf("%q: missed documented callers %v", entry.Query, cmp.FalseNegatives)
			}
			if len(cmp.FalsePositives) != 0 {
				t.Errorf("%q: reported non-callers %v", entry.Query, cmp.FalsePositives)
			}
			if len(cmp.StaleKnownFalsePositives) != 0 {
				t.Errorf("%q: known false positives %v no longer appear; drop them from truth.json", entry.Query, cmp.StaleKnownFalsePositives)
			}
			if len(cmp.StaleKnownFalseNegatives) != 0 {
				t.Errorf("%q: known false negatives %v now appear; drop them from truth.json", entry.Query, cmp.StaleKnownFalseNegatives)
			}
		})
	}

	measured := totalTP + totalFP
	precision := 0.0
	if measured > 0 {
		precision = float64(totalTP) / float64(measured)
	}
	recall := 1.0
	if totalTP+totalFN > 0 {
		recall = float64(totalTP) / float64(totalTP+totalFN)
	}
	t.Logf("corpus=%s tp=%d fp=%d fn=%d precision=%.2f recall=%.2f", truth.Language, totalTP, totalFP, totalFN, precision, recall)
}

// buildScouterForImpact builds the CLI once per test and returns the binary path
// with a config file that points the store at a throwaway database.
// scouterCLI is a freshly built binary plus the throwaway store it points at.
type scouterCLI struct {
	bin        string
	configPath string
	dbPath     string
}

func buildScouterForImpact(t *testing.T, repoRoot string) scouterCLI {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "scouter")
	build := exec.Command("go", "build", "-o", bin, "./cmd/scouter")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build scouter: %v\n%s", err, out)
	}
	configPath := filepath.Join(dir, "config.toml")
	dbPath := filepath.Join(dir, "scouter.db")
	config := fmt.Sprintf("[tracking]\ndb_path = %q\n", dbPath)
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return scouterCLI{bin: bin, configPath: configPath, dbPath: dbPath}
}

// runScouter runs one CLI invocation and returns its combined output.
func runScouter(t *testing.T, bin, configPath string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "SCOUTER_CONFIG="+configPath)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// parseImpactCallers reads the caller names out of the CLI rendering.
func parseImpactCallers(out string) []string {
	var callers []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		name := strings.TrimPrefix(line, "- ")
		if idx := strings.Index(name, " ("); idx >= 0 {
			name = name[:idx]
		}
		callers = append(callers, strings.TrimSpace(name))
	}
	return callers
}

func TestNormalizeGoCaller(t *testing.T) {
	tests := []struct{ in, want string }{
		{"github.com/x/y/pkg.Function", "Function"},
		{"github.com/x/y/pkg.Recv.Method", "Method"},
		{"github.com/x/y/pkg.Recv.Method.func1", "Method"},
		{"github.com/x/y/pkg.TestThing.func2", "TestThing"},
		{"Bare", "Bare"},
	}
	for _, tt := range tests {
		if got := normalizeGoCaller(tt.in); got != tt.want {
			t.Errorf("normalizeGoCaller(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// normalizeGoCaller reduces a caller as the index renders it to the name of the
// function that contains the call site. Closures collapse onto their named
// parent, so the measurement compares against what a reader can point at in the
// source instead of against the index's internal naming of a literal.
func normalizeGoCaller(name string) string {
	name = goClosureSuffix.ReplaceAllString(name, "")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

var goClosureSuffix = regexp.MustCompile(`\.func\d+$`)

// goCallSite is one call edge as the source shows it: the file holding the call
// and the top-level function that contains it. Closures are not named, because a
// reader points at the function, not at a literal.
type goCallSite struct {
	File     string
	Function string
}

func (c goCallSite) key() string { return c.File + "|" + c.Function }

var (
	goFuncDecl    = regexp.MustCompile(`^func (?:\([^)]*\) )?([A-Za-z0-9_]+)\(`)
	goStringParts = regexp.MustCompile(`"[^"]*"|` + "`[^`]*`" + `|'[^']*'`)
	goSkippedDirs = map[string]bool{".git": true, "vendor": true, "node_modules": true, "testdata": true}
)

// goCallerOracle answers "who calls this symbol" by reading the Go sources under
// root. It never opens the index: comments, string literals and the declaration
// itself are excluded, and each remaining call site is attributed to the
// top-level function that encloses it.
func goCallerOracle(t *testing.T, root, symbol string) []goCallSite {
	t.Helper()
	callPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\(`)
	declPattern := regexp.MustCompile(`^func (?:\([^)]*\) )?` + regexp.QuoteMeta(symbol) + `\(`)
	var sites []goCallSite

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if goSkippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lines := strings.Split(string(raw), "\n")
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
				continue
			}
			if declPattern.MatchString(line) {
				continue
			}
			if !callPattern.MatchString(goStringParts.ReplaceAllString(line, `""`)) {
				continue
			}
			fn := enclosingGoFunction(lines, i)
			if fn == "" {
				continue
			}
			sites = append(sites, goCallSite{File: filepath.ToSlash(rel), Function: fn})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return sites
}

// enclosingGoFunction finds the top-level function or method that contains line.
func enclosingGoFunction(lines []string, index int) string {
	for i := index; i >= 0; i-- {
		if m := goFuncDecl.FindStringSubmatch(lines[i]); m != nil {
			return m[1]
		}
	}
	return ""
}

// TestImpactPrecision_GoIndex measures the same question as the Python corpus,
// but on real Go code and one layer down: it reads the direct call edges the
// indexer wrote and compares them with the ones the source shows. The CLI answers
// with the transitive blast radius rendered as fully-qualified names, which no
// hand-written key of direct callers can be compared against, so the edge layer
// is where a Go figure can be honest. Indexing this repository costs seconds, so
// the measurement is opt-in.
func TestImpactPrecision_GoIndex(t *testing.T) {
	if os.Getenv("SCOUTER_IMPACT_SELF") == "" {
		t.Skip("set SCOUTER_IMPACT_SELF=1 to measure direct Go call edges against this repository")
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	sc := buildScouterForImpact(t, repoRoot)
	if out, err := runScouter(t, sc.bin, sc.configPath, "index", repoRoot); err != nil {
		t.Fatalf("index failed: %v\n%s", err, out)
	}

	raw, err := os.ReadFile(filepath.Join(wd, "fixtures", "impact_truth", "go", "truth.json"))
	if err != nil {
		t.Fatalf("read truth: %v", err)
	}
	var truth impactTruth
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatalf("parse truth: %v", err)
	}

	ctx := context.Background()
	st, err := store.NewStore(ctx, sc.dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	totalTP, totalFP, totalFN := 0, 0, 0
	for _, entry := range truth.Symbols {
		t.Run(entry.Query, func(t *testing.T) {
			want := goCallerOracle(t, repoRoot, entry.Query)
			got, err := indexedCallSites(ctx, st, repoRoot, entry.Query)
			if err != nil {
				t.Fatalf("%q: %v", entry.Query, err)
			}

			cmp := compareImpactCallers(keysOf(want), keysOf(got), entry.KnownFalsePositives, entry.KnownFalseNegatives)
			t.Logf("%-14s source=%d indexed=%d tp=%d fp=%v fn=%v known_fp=%v known_fn=%v",
				entry.Query, len(want), len(got), len(cmp.TruePositives), cmp.FalsePositives, cmp.FalseNegatives,
				cmp.AbsorbedFalsePositives, cmp.AbsorbedFalseNegatives)

			totalTP += len(cmp.TruePositives)
			totalFP += len(cmp.FalsePositives) + len(cmp.AbsorbedFalsePositives)
			totalFN += len(cmp.FalseNegatives) + len(cmp.AbsorbedFalseNegatives)

			if len(cmp.FalseNegatives) != 0 {
				t.Errorf("%q: the index is missing call edges the source shows: %v", entry.Query, cmp.FalseNegatives)
			}
			if len(cmp.FalsePositives) != 0 {
				t.Errorf("%q: the index invented call edges the source does not show: %v", entry.Query, cmp.FalsePositives)
			}
			if len(cmp.StaleKnownFalsePositives) != 0 || len(cmp.StaleKnownFalseNegatives) != 0 {
				t.Errorf("%q: ratchet entries are stale and must be dropped: %v %v",
					entry.Query, cmp.StaleKnownFalsePositives, cmp.StaleKnownFalseNegatives)
			}
		})
	}

	measured := totalTP + totalFP
	precision := 1.0
	if measured > 0 {
		precision = float64(totalTP) / float64(measured)
	}
	recall := 1.0
	if totalTP+totalFN > 0 {
		recall = float64(totalTP) / float64(totalTP+totalFN)
	}
	t.Logf("corpus=%s edges=%d tp=%d fp=%d fn=%d precision=%.2f recall=%.2f",
		truth.Language, totalTP+totalFP, totalTP, totalFP, totalFN, precision, recall)
}

// indexedCallSites returns the direct callers the index holds for symbol.
func indexedCallSites(ctx context.Context, st store.Store, repoRoot, symbol string) ([]goCallSite, error) {
	syms, err := st.SearchSymbols(ctx, symbol, "", "", 50, 0)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", symbol, err)
	}
	seen := map[string]bool{}
	var fqns []string
	for _, s := range syms {
		if s.Name != symbol {
			continue
		}
		fqn := s.Name
		if s.PackagePath != "" {
			fqn = s.PackagePath + "." + s.Name
		}
		if !seen[fqn] {
			seen[fqn] = true
			fqns = append(fqns, fqn)
		}
	}
	if len(fqns) != 1 {
		return nil, fmt.Errorf("%q resolves to %d symbols (%v): pick one definition to measure", symbol, len(fqns), fqns)
	}

	calls, err := st.GetCallers(ctx, fqns[0], 0, 0)
	if err != nil {
		return nil, fmt.Errorf("callers of %q: %w", fqns[0], err)
	}
	sites := make([]goCallSite, 0, len(calls))
	for _, c := range calls {
		rel, relErr := filepath.Rel(repoRoot, c.Path)
		if relErr != nil {
			rel = c.Path
		}
		sites = append(sites, goCallSite{File: filepath.ToSlash(rel), Function: normalizeGoCaller(c.CallerName)})
	}
	return sites, nil
}

func keysOf(sites []goCallSite) []string {
	keys := make([]string, 0, len(sites))
	for _, s := range sites {
		keys = append(keys, s.key())
	}
	return keys
}
