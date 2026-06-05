package display

import (
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
)

// ExportMermaid formats a slice of calls into a Mermaid graph string.
func ExportMermaid(calls []store.Call, title string) string {
	var b strings.Builder
	b.WriteString("graph TD\n")
	if title != "" {
		b.WriteString(fmt.Sprintf("  subgraph %q\n", title))
	}

	// Use a map to deduplicate edges and sanitize names
	seen := make(map[string]bool)

	sanitize := func(s string) string {
		// Mermaid node IDs shouldn't have dots or slashes unless quoted
		// But quoting is safer.
		return fmt.Sprintf("%q", s)
	}

	for _, c := range calls {
		caller := sanitize(c.CallerName)
		callee := sanitize(c.CalleeName)
		edge := fmt.Sprintf("  %s --> %s", caller, callee)

		if !seen[edge] {
			b.WriteString(edge + "\n")
			seen[edge] = true
		}
	}

	if title != "" {
		b.WriteString("  end\n")
	}

	return b.String()
}
