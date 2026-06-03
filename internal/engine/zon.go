package engine

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// EncodeZON implements a minimal ZON (Zero Overhead Notation) table encoder.
// It converts slices of structs into a token-efficient tabular format for LLMs,
// saving 35-70% of tokens compared to JSON.
func EncodeZON(slice interface{}) (string, error) {
	val := reflect.ValueOf(slice)
	if val.Kind() != reflect.Slice {
		return "", fmt.Errorf("ZON encoder requires a slice")
	}
	if val.Len() == 0 {
		return "@(0)[]", nil
	}

	elemType := val.Index(0).Type()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	if elemType.Kind() != reflect.Struct {
		return "", fmt.Errorf("ZON encoder requires a slice of structs")
	}

	var sb strings.Builder
	// ZON Header: @(count)[col1 | col2]
	fmt.Fprintf(&sb, "@(%d)[", val.Len())
	for i := 0; i < elemType.NumField(); i++ {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(elemType.Field(i).Name)
	}
	sb.WriteString("]\n")

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		for j := 0; j < item.NumField(); j++ {
			if j > 0 {
				sb.WriteString(" | ")
			}
			fmt.Fprintf(&sb, "%v", item.Field(j).Interface())
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// EncodeZONNeighborhood formats a 1-hop structural neighborhood (imports, exports, calls) into ZON format.
func EncodeZONNeighborhood(filePath string, imports []string, exports []store.Symbol, calls []store.Call) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[ZON Neighborhood: %s]\n", filePath))

	if len(imports) > 0 {
		for _, imp := range imports {
			sb.WriteString(fmt.Sprintf("IMP | %s\n", imp))
		}
	} else {
		sb.WriteString("IMP | (none)\n")
	}

	if len(exports) > 0 {
		for _, exp := range exports {
			sb.WriteString(fmt.Sprintf("EXP | %s | %s\n", exp.Type, exp.Name))
		}
	} else {
		sb.WriteString("EXP | (none)\n")
	}

	if len(calls) > 0 {
		for _, call := range calls {
			sb.WriteString(fmt.Sprintf("CALL | %s -> %s\n", call.CallerName, call.CalleeName))
		}
	} else {
		sb.WriteString("CALL | (none)\n")
	}

	return sb.String()
}

func EncodeZONImpact(res *types.ImpactResult) string {
	if res == nil || res.Target.Symbol == "" {
		return "[ZON Impact: (no impact results)]"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[ZON Impact: %s (Risk: %.4f %s)]\n", res.Target.Symbol, res.Target.RiskScore, res.RiskLevel))

	if len(res.Callers) > 0 {
		for _, caller := range res.Callers {
			sb.WriteString(fmt.Sprintf("CALLER | %s | %s | dist:%d\n", caller.Symbol, caller.File, caller.Distance))
		}
	}

	if res.Mermaid != "" {
		sb.WriteString(res.Mermaid + "\n")
	}

	return sb.String()
}

func EncodeZONPredict(tests []types.TestTarget) string {
	if len(tests) == 0 {
		return "[ZON Predict: (no tests affected)]"
	}
	var sb strings.Builder
	sb.WriteString("[ZON Predict]\n")
	for _, test := range tests {
		sb.WriteString(fmt.Sprintf("TEST | %s | %s\n", test.Name, test.File))
	}
	return sb.String()
}

func EncodeZONVerify(diff *ChronosDiff) string {
	if diff == nil {
		return "[ZON Verify: (no diff found)]"
	}
	var sb strings.Builder
	sb.WriteString("[ZON Verify]\n")
	for _, name := range diff.MissingSymbols {
		sb.WriteString(fmt.Sprintf("MISSING | symbol | %s\n", name))
	}
	for _, name := range diff.MangledSymbols {
		sb.WriteString(fmt.Sprintf("MANGLED | symbol | %s\n", name))
	}
	for _, name := range diff.AddedSymbols {
		sb.WriteString(fmt.Sprintf("ADDED | symbol | %s\n", name))
	}
	return sb.String()
}

func EncodeZONLSPLocation(locs []lsp.Location) (string, error) {
	if len(locs) == 0 {
		return "[ZON Location: (none found)]", nil
	}
	var sb strings.Builder
	sb.WriteString("[ZON Location]\n")
	for _, loc := range locs {
		sb.WriteString(fmt.Sprintf("LOC | %s | line:%d\n", loc.URI, loc.Range.Start.Line+1))
	}
	return sb.String(), nil
}

func EncodeZONLSPHover(hover *lsp.Hover) (string, error) {
	if hover == nil || hover.Contents.Value == "" {
		return "[ZON Hover: (no documentation found)]", nil
	}
	var sb strings.Builder
	sb.WriteString("[ZON Hover]\n")
	sb.WriteString(fmt.Sprintf("DOC | %s\n", hover.Contents.Value))
	return sb.String(), nil
}
