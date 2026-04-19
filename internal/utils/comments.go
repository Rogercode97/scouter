package utils

import (
	"strings"
)

// CleanComment strips comment markers and normalizes docstrings across Go, TS, and Python.
func CleanComment(raw string) string {
	if raw == "" {
		return ""
	}

	lines := strings.Split(raw, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 1. Strip block starts/ends
		trimmed = strings.TrimPrefix(trimmed, "/**")
		trimmed = strings.TrimPrefix(trimmed, "/*")
		trimmed = strings.TrimSuffix(trimmed, "*/")
		trimmed = strings.TrimPrefix(trimmed, `"""`)
		trimmed = strings.TrimSuffix(trimmed, `"""`)
		trimmed = strings.TrimPrefix(trimmed, `'''`)
		trimmed = strings.TrimSuffix(trimmed, `'''`)

		// 2. Strip line markers: //, #, *
		if strings.HasPrefix(trimmed, "//") {
			trimmed = strings.TrimPrefix(trimmed, "//")
		} else if strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimPrefix(trimmed, "#")
		} else if strings.HasPrefix(trimmed, "*") {
			trimmed = strings.TrimPrefix(trimmed, "*")
		}

		trimmed = strings.TrimSpace(trimmed)

		// 3. Collect non-empty lines, keeping internal spacing
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	return strings.Join(cleaned, "\n")
}
