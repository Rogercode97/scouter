package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/types"
)

// ASTRuleEngine executes architectural rules using ast-grep.
type ASTRuleEngine struct {
	rulesDir string
}

// NewASTRuleEngine creates a new engine pointing to the rules directory.
func NewASTRuleEngine(rulesDir string) *ASTRuleEngine {
	return &ASTRuleEngine{
		rulesDir: rulesDir,
	}
}

// Audit runs all rules in the rules directory against a specific file or directory.
func (e *ASTRuleEngine) Audit(ctx context.Context, targetPath string) ([]types.ASTRuleMatch, error) {
	if _, err := exec.LookPath("sg"); err != nil {
		if _, err := exec.LookPath("ast-grep"); err != nil {
			return nil, fmt.Errorf("ast-grep (sg) not found in PATH")
		}
	}

	rules, err := e.GetRules()
	if err != nil {
		return nil, err
	}

	var allMatches []types.ASTRuleMatch
	for _, ruleFile := range rules {
		rulePath := filepath.Join(e.rulesDir, ruleFile)

		// ast-grep scan -r <rule_file> <target> --json
		cmd := exec.CommandContext(ctx, "sg", "scan", "-r", rulePath, targetPath, "--json")

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		// ast-grep returns exit code 1 if matches are found, which is not a "command error" for us
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				return nil, fmt.Errorf("ast-grep execution failed for rule %s: %w (stderr: %s)", ruleFile, err, stderr.String())
			}
		}

		if stdout.Len() == 0 {
			continue
		}

		var matches []types.ASTRuleMatch
		if err := json.Unmarshal(stdout.Bytes(), &matches); err != nil {
			// Some versions might output nothing or different format on no match
			continue
		}
		allMatches = append(allMatches, matches...)
	}

	return allMatches, nil
}

// GetRules returns a list of available rule files.
func (e *ASTRuleEngine) GetRules() ([]string, error) {
	files, err := os.ReadDir(e.rulesDir)
	if err != nil {
		return nil, err
	}

	var rules []string
	for _, f := range files {
		if !f.IsDir() && (filepath.Ext(f.Name()) == ".yaml" || filepath.Ext(f.Name()) == ".yml") {
			rules = append(rules, f.Name())
		}
	}
	return rules, nil
}
