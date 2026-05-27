package engine

import (
	"fmt"
	"reflect"
	"strings"
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
