package engine

import (
	"os"
	"testing"
)

func TestLedger_BudgetEnforcement(t *testing.T) {
	l := NewLedger()
	l.SetLedgerPath("test_budget_ledger.json")
	defer os.Remove("test_budget_ledger.json")

	l.SetBudget(100, 2) // Very tight budget: 100 Ki, 2 turns

	t.Run("Respect Turn Limit", func(t *testing.T) {
		p1 := Patch{FilePath: "f1.go", NewContent: "content1"}
		l.IncrementTurn()
		if err := l.Stage(p1.FilePath, p1); err != nil {
			t.Fatalf("Stage 1 failed: %v", err)
		}

		p2 := Patch{FilePath: "f2.go", NewContent: "content2"}
		l.IncrementTurn()
		if err := l.Stage(p2.FilePath, p2); err != nil {
			t.Fatalf("Stage 2 failed: %v", err)
		}

		p3 := Patch{FilePath: "f3.go", NewContent: "content3"}
		l.IncrementTurn()
		if err := l.Stage(p3.FilePath, p3); err == nil {
			t.Error("Expected error when exceeding turn limit, but got nil")
		}
	})

	t.Run("Respect Ki Limit", func(t *testing.T) {
		l2 := NewLedger()
		l2.SetLedgerPath("test_ki_ledger.json")
		defer os.Remove("test_ki_ledger.json")

		l2.SetBudget(10, 10) // 10 Ki limit

		// 44 chars ≈ 11 Ki (exceeds 10)
		largePatch := Patch{FilePath: "large.go", NewContent: "This content is definitely longer than forty characters"}
		if err := l2.Stage(largePatch.FilePath, largePatch); err == nil {
			t.Error("Expected error when exceeding Ki limit, but got nil")
		}
	})
}
