package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
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
	truth := NewTruthEngine(db, WithASTRules(astEngine))

	// 3. Action: Index the violating file
	err = truth.Index(ctx, violatingFile)
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
