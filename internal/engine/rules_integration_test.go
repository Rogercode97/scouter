package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASTRuleIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Setup temporary workspace
	tempDir, err := os.MkdirTemp("", "scouter-rules-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a rule directory with the domain-isolation rule
	rulesDir := filepath.Join(tempDir, "rules")
	err = os.Mkdir(rulesDir, 0755)
	require.NoError(t, err)

	ruleContent := `id: domain-isolation
language: go
rule:
  pattern: import "database/sql"
message: "Hexagonal Violation detected"
severity: error`
	err = os.WriteFile(filepath.Join(rulesDir, "domain-isolation.yaml"), []byte(ruleContent), 0644)
	require.NoError(t, err)

	// Create a violating file
	violatingFile := filepath.Join(tempDir, "domain_logic.go")
	badCode := `package domain
import "database/sql"
func DoSomething() {}`
	err = os.WriteFile(violatingFile, []byte(badCode), 0644)
	require.NoError(t, err)

	// 2. Initialize Engines
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := store.NewStore(ctx, dbPath)
	require.NoError(t, err)
	defer db.Close()

	astEngine := NewASTRuleEngine(rulesDir)
	indexer := NewIndexerPipeline(IndexerConfig{Store: db, ASTRules: astEngine})
	err = indexer.Index(ctx, violatingFile)
	require.NoError(t, err)

	// 4. Verify: Violation should be in the database
	violations, err := db.GetViolationsByFile(ctx, violatingFile)
	require.NoError(t, err)

	if len(violations) == 0 {
		t.Logf("DIAGNOSTIC: No violations found for %s", violatingFile)
		// Check if rules directory is correctly perceived
		t.Logf("DIAGNOSTIC: Rules directory: %s", rulesDir)
		if files, err := os.ReadDir(rulesDir); err == nil {
			for _, f := range files {
				t.Logf("DIAGNOSTIC: Rule file: %s", f.Name())
			}
		}
	}

	require.NotEmpty(t, violations, "Should have found a violation")
	assert.Equal(t, "domain-isolation", violations[0].RuleID)
	assert.Equal(t, "error", violations[0].Severity)
}

// TestASTRuleEngine_MalformedRuleSurfaces is the regression guard for a gate
// that reported "no violations" while every rule file failed to parse: a
// non-zero exit that is not the match signal must surface as an error.
func TestASTRuleEngine_MalformedRuleSurfaces(t *testing.T) {
	if _, err := utils.AstGrepBinary(); err != nil {
		t.Skip("ast-grep not installed")
	}

	dir := t.TempDir()
	rulesDir := filepath.Join(dir, "rules")
	require.NoError(t, os.Mkdir(rulesDir, 0o755))

	// severity must be lowercase; this spelling is what ast-grep rejects.
	bad := "id: broken\nlanguage: go\nseverity: ERROR\nrule:\n  pattern: panic($$$)\nmessage: boom\n"
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "broken.yaml"), []byte(bad), 0o644))

	target := filepath.Join(dir, "target.go")
	require.NoError(t, os.WriteFile(target, []byte("package p\n\nfunc Boom() { panic(\"x\") }\n"), 0o644))

	engine := NewASTRuleEngine(rulesDir)
	matches, err := engine.Audit(context.Background(), target)
	if err == nil {
		t.Fatalf("a rule that fails to parse must surface an error, got %d matches and nil", len(matches))
	}
}

// TestASTRuleEngine_FiresOnViolation proves the gate can actually block: a
// valid rule against code that violates it must return a match.
func TestASTRuleEngine_FiresOnViolation(t *testing.T) {
	if _, err := utils.AstGrepBinary(); err != nil {
		t.Skip("ast-grep not installed")
	}

	dir := t.TempDir()
	rulesDir := filepath.Join(dir, "rules")
	require.NoError(t, os.Mkdir(rulesDir, 0o755))

	good := "id: no-panic\nlanguage: go\nseverity: error\nrule:\n  pattern: panic($$$)\nmessage: use errors\n"
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "no-panic.yaml"), []byte(good), 0o644))

	target := filepath.Join(dir, "target.go")
	require.NoError(t, os.WriteFile(target, []byte("package p\n\nfunc Boom() { panic(\"x\") }\n"), 0o644))

	engine := NewASTRuleEngine(rulesDir)
	matches, err := engine.Audit(context.Background(), target)
	require.NoError(t, err)
	require.NotEmpty(t, matches, "a panic() must violate the no-panic rule")
	assert.Equal(t, "no-panic", matches[0].RuleID)
}
