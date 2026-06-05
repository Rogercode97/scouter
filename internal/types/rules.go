package types

import "fmt"

// ASTRuleMatch represents a single violation found by ast-grep.
type ASTRuleMatch struct {
	RuleID   string   `json:"ruleId"`
	File     string   `json:"file"`
	Text     string   `json:"text"`
	Message  string   `json:"message"`
	Severity string   `json:"severity"`
	Range    ASTRange `json:"range"`
}

// ASTRange represents the location of a match in the source code.
type ASTRange struct {
	Start ASTPos `json:"start"`
	End   ASTPos `json:"end"`
}

// ASTPos represents a line and column position.
type ASTPos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (m ASTRuleMatch) String() string {
	return fmt.Sprintf("[%s] %s:%d:%d: %s", m.Severity, m.File, m.Range.Start.Line+1, m.Range.Start.Column+1, m.Message)
}

// RuleViolationReport is a collection of matches for a specific file.
type RuleViolationReport struct {
	FilePath   string
	Violations []ASTRuleMatch
}
