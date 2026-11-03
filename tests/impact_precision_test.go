package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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

	bin, configPath := buildScouterForImpact(t, repoRoot)

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
func buildScouterForImpact(t *testing.T, repoRoot string) (string, string) {
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
	config := fmt.Sprintf("[tracking]\ndb_path = %q\n", filepath.Join(dir, "scouter.db"))
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return bin, configPath
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
