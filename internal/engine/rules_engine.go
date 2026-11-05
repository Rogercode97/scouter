package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
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
	binary, err := utils.AstGrepBinary()
	if err != nil {
		return nil, err
	}

	rules, err := e.GetRules()
	if err != nil {
		return nil, err
	}

	var allMatches []types.ASTRuleMatch
	for _, ruleFile := range rules {
		rulePath := filepath.Join(e.rulesDir, ruleFile)

		// ast-grep scan --rule <rule_file> <target> --json. Routed through
		// SafeCommand like every other external invocation: the rule path and
		// the target both arrive from user input.
		cmd, err := utils.SafeCommand(ctx, binary, "scan", "--rule", rulePath, targetPath, "--json")
		if err != nil {
			return nil, fmt.Errorf("ast-grep scan for rule %s: %w", ruleFile, err)
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		// ast-grep exits 1 when it found matches, which is success here. Any
		// other non-zero exit is a real failure: reporting it as "no matches"
		// is what let a malformed rule file silence this gate.
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return nil, fmt.Errorf("ast-grep execution failed for rule %s: %w (stderr: %s)", ruleFile, err, stderr.String())
			}
			if code := exitErr.ExitCode(); code != 1 {
				return nil, fmt.Errorf("ast-grep scan failed for rule %s (exit %d): %s", ruleFile, code, strings.TrimSpace(stderr.String()))
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
