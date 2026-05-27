package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestArchitecturalAuditIntegration(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer repo.Close()

	// Setup Rules Engine
	// We need to point to the actual rules directory.
	// Since tests run in the package directory, we go up two levels.
	rulesDir := filepath.Join("..", "..", "internal", "filters", "rules")
	ruleEngine := NewASTRuleEngine(rulesDir)

	// Setup TruthEngine with RuleEngine
	engine := NewTruthEngine(repo, WithASTRules(ruleEngine))

	// Create a file that violates domain isolation
	violationFile := filepath.Join(tmpDir, "domain_violation.go")
	content := `package domain
import "database/sql"
func BadFunction() {
	println("I am breaking hexagonal architecture")
}`
	err = os.WriteFile(violationFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write violation file: %v", err)
	}

	// Index the file (this should trigger the audit)
	err = engine.Index(ctx, violationFile)
	if err != nil {
		t.Fatalf("Failed to index file: %v", err)
	}

	// Verify violation was persisted
	violations, err := repo.GetViolationsByFile(ctx, violationFile)
	if err != nil {
		t.Fatalf("Failed to get violations: %v", err)
	}

	if len(violations) == 0 {
		t.Errorf("Expected at least one violation, found 0")
	}

	found := false
	for _, v := range violations {
		if v.RuleID == "domain-isolation" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected 'domain-isolation' violation not found")
	}
}
