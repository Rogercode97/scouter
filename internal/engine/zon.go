package engine

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
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

	var headers []string
	for i := 0; i < elemType.NumField(); i++ {
		headers = append(headers, elemType.Field(i).Name)
	}

	var sb strings.Builder
	// ZON Header: @(count)[col1 | col2]
	sb.WriteString(fmt.Sprintf("@(%d)[%s]\n", val.Len(), strings.Join(headers, " | ")))

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		var row []string
		for j := 0; j < item.NumField(); j++ {
			row = append(row, fmt.Sprintf("%v", item.Field(j).Interface()))
		}
		sb.WriteString(strings.Join(row, " | ") + "\n")
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
