package filter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// AstGrepFilter implements structural filtering using ast-grep (sg).
type AstGrepFilter struct {
	binaryPath string
	once       sync.Once
}

var astGrep = &AstGrepFilter{}

// astGrepAction is the bound ActionFunc for ast-grep.
func astGrepAction(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	return astGrep.Apply(ctx, input, params)
}

// AstGrepMatch represents a single match from ast-grep --json=stream.
type AstGrepMatch struct {
	Text        string `json:"text"`
	File        string `json:"file"`
	Replacement string `json:"replacement,omitempty"`
	Lines       string `json:"lines"`
	Range       struct {
		Start struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"start"`
		End struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"end"`
	} `json:"range"`
}

// Apply executes ast-grep against the input lines.
func (a *AstGrepFilter) Apply(ctx context.Context, input ActionResult, params map[string]any) (ActionResult, error) {
	a.once.Do(func() {
		// Discover binary: prefer 'sg' over 'ast-grep'
		path, err := exec.LookPath("sg")
		if err != nil {
			path, _ = exec.LookPath("ast-grep")
		}
		a.binaryPath = path
	})

	binary := a.binaryPath
	if override := getStr(params, "binary"); override != "" {
		if p, err := exec.LookPath(override); err == nil {
			binary = p
		} else {
			binary = override
		}
	}

	if binary == "" {
		return input, fmt.Errorf("ast-grep binary not found (tried 'sg' and 'ast-grep')")
	}

	pattern := getStr(params, "pattern")
	if pattern == "" {
		return input, fmt.Errorf("ast_grep: 'pattern' is required")
	}

	rewrite := getStr(params, "rewrite")
	lang := getStr(params, "lang")
	if lang == "" {
		lang = a.detectLang(input.Metadata)
	}

	// Prepare command
	// For rewrites, we might want the whole transformed output.
	// However, if we follow the pipeline pattern, we might want to stay in JSON-land if possible.
	// But ast-grep doesn't output the full rewritten file in JSON stream mode, only the replacements.
	
	args := []string{"run", "--pattern", pattern}
	
	searchPath := getStr(params, "path")
	useStdin := searchPath == "" && len(input.Lines) > 0

	if useStdin {
		args = append(args, "--stdin")
	}

	if rewrite != "" {
		args = append(args, "--rewrite", rewrite)
	}

	// Optimization: ast-grep performs better when the language is explicitly defined, 
	// as it allows the internal 'potential_kinds' heuristic to trigger early.
	// For --stdin, --lang is highly recommended to avoid parsing ambiguity.
	if lang != "" {
		args = append(args, "--lang", lang)
	} else if useStdin {
		// Default to generic or let it fail? For now, we attempt to detect but 
		// if we can't, ast-grep might struggle with --stdin.
	}

	// If it's a rewrite, we don't use --json=stream to get the whole file back.
	// If it's a search, we use --json=stream to get matches.
	useJSON := rewrite == ""
	if useJSON {
		args = append(args, "--json=stream")
	}

	if searchPath != "" {
		args = append(args, searchPath)
	}

	cmd := exec.CommandContext(ctx, binary, args...)

	if useStdin {
		// Pipe stdin
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return input, fmt.Errorf("ast_grep: stdin pipe: %w", err)
		}

		go func() {
			defer stdin.Close()
			io.WriteString(stdin, strings.Join(input.Lines, "\n"))
		}()
	}

	// Capture stderr for debugging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return input, fmt.Errorf("ast_grep: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return input, fmt.Errorf("ast_grep: start: %w (stderr: %s)", err, stderr.String())
	}

	if !useJSON {
		// For rewrite, we still need to read the whole thing as it represents the transformed file.
		var result bytes.Buffer
		if _, err := io.Copy(&result, stdout); err != nil {
			return input, fmt.Errorf("ast_grep: read rewrite output: %w", err)
		}

		if err := cmd.Wait(); err != nil {
			if cmd.ProcessState.ExitCode() != 1 {
				return input, fmt.Errorf("ast_grep: %w (stderr: %s)", err, stderr.String())
			}
		}

		res := result.String()
		if res == "" && len(input.Lines) > 0 {
			return input, nil
		}
		input.Lines = strings.Split(strings.TrimRight(res, "\n"), "\n")
		return input, nil
	}

	// For search, parse JSON stream
	newLines, err := a.parseJSONStream(stdout)
	if err != nil {
		return input, err
	}

	if err := cmd.Wait(); err != nil {
		// Exit code 1 usually means no matches in ast-grep
		if cmd.ProcessState.ExitCode() != 1 {
			return input, fmt.Errorf("ast_grep: %w (stderr: %s)", err, stderr.String())
		}
	}

	if len(newLines) > 0 {
		input.Lines = newLines
	} else {
		input.Lines = []string{}
	}

	return input, nil
}

// parseJSONStream parses NDJSON output from ast-grep, skipping malformed lines.
func (a *AstGrepFilter) parseJSONStream(r io.Reader) ([]string, error) {
	var newLines []string
	seenLines := make(map[int]bool)
	
	// Use a scanner to read NDJSON line-by-line for resiliency
	scanner := bufio.NewScanner(r)
	// Increase buffer size for very long lines in large files
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var match AstGrepMatch
		if err := json.Unmarshal(line, &match); err != nil {
			// Skip malformed JSON lines
			continue
		}

		// Collect matched lines
		matchLines := strings.Split(match.Lines, "\n")
		for i, lineContent := range matchLines {
			lineNum := match.Range.Start.Line + i
			if !seenLines[lineNum] {
				newLines = append(newLines, lineContent)
				seenLines[lineNum] = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ast_grep: scan output: %w", err)
	}

	return newLines, nil
}

func (a *AstGrepFilter) detectLang(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	// Try to get filename or path from metadata
	filePath, _ := metadata["file"].(string)
	if filePath == "" {
		filePath, _ = metadata["path"].(string)
	}
	if filePath == "" {
		return ""
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".jsx":
		return "jsx"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cpp", ".cc", ".cxx", ".h", ".hpp":
		return "cpp"
	case ".c":
		return "c"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".html":
		return "html"
	case ".css":
		return "css"
	default:
		return ""
	}
}
