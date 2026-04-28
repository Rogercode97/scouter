package utils

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

var jsonRegex = regexp.MustCompile(`(?s)([\[\{].*[\]\}])`)

// ExtractJSON isolates a JSON block from potentially conversational text.
func ExtractJSON(s string) string {
	match := jsonRegex.FindString(s)
	if match == "" {
		return s
	}
	return match
}

var codeBlockRegex = regexp.MustCompile("(?s)```(?:[a-zA-Z0-9]+)?\n?(.*?)\n?```")

// ExtractCodeBlock isolates code from Markdown blocks or returns the raw string if no blocks found.
// Unlike ExtractJSON, it does not attempt to match braces, making it safe for Go code.
func ExtractCodeBlock(s string) string {
	match := codeBlockRegex.FindStringSubmatch(s)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return strings.TrimSpace(s)
}

var ansiRe = NewLazyRegex(`\x1b\[[0-9;]*[a-zA-Z]`)

// Truncate truncates s to max runes, appending "..." if truncated.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// StripANSI removes ANSI escape codes from s.
func StripANSI(s string) string {
	return ansiRe.Re().ReplaceAllString(s, "")
}

// EstimateTokens estimates token count using ~4 chars/token heuristic.
func EstimateTokens(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	return int(math.Ceil(float64(n) / 4.0))
}

// FormatTokens formats a token count for display: "1.2M", "59.2K", "694".
func FormatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// CountLines counts the number of lines in s.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// CompactPath strips common prefixes like src/, lib/, internal/ from a path.
func CompactPath(path string) string {
	prefixes := []string{"src/", "lib/", "internal/", "pkg/", "vendor/"}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return path[len(p):]
		}
	}
	return path
}

// OkConfirmation produces a compact confirmation message.
func OkConfirmation(action, detail string) string {
	if detail == "" {
		return "ok " + action
	}
	return fmt.Sprintf("ok %s %s", action, detail)
}

var ratingRegex = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*/\s*10`)

// ParseRating extracts a rating like "8.5 / 10" from text.
func ParseRating(s string) (float64, error) {
	match := ratingRegex.FindStringSubmatch(s)
	if len(match) < 2 {
		return 0, fmt.Errorf("rating not found")
	}
	var r float64
	_, err := fmt.Sscanf(match[1], "%f", &r)
	return r, err
}

// ExtractList extracts a bulleted list following a specific header.
func ExtractList(text, header string) []string {
	lines := strings.Split(text, "\n")
	var result []string
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), strings.ToLower(header)) {
			found = true
			continue
		}
		if found {
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
				item := strings.TrimSpace(trimmed[1:])
				if item != "" {
					result = append(result, item)
				}
			} else if len(result) > 0 {
				// Stop if we encounter a non-list line after finding items
				break
			}
		}
	}
	return result
}
