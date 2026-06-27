package mcp

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// EncodeCursor creates a base64 encoded cursor from offset and limit.
func EncodeCursor(offset int, limit int) string {
	raw := fmt.Sprintf("%d|%d", offset, limit)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor decodes a base64 cursor string into offset and limit.
func DecodeCursor(cursor string) (offset int, limit int, err error) {
	if cursor == "" {
		return 0, 0, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor encoding: %v", err)
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid cursor format")
	}

	offset, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor offset: %v", err)
	}

	limit, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor limit: %v", err)
	}

	return offset, limit, nil
}
