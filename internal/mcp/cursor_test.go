package mcp

import (
	"testing"
)

func TestCursorEncodingDecoding(t *testing.T) {
	offset := 50
	limit := 25

	cursor := EncodeCursor(offset, limit)
	if cursor == "" {
		t.Fatal("expected cursor to be non-empty")
	}

	decOffset, decLimit, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("failed to decode cursor: %v", err)
	}

	if decOffset != offset {
		t.Errorf("expected offset %d, got %d", offset, decOffset)
	}
	if decLimit != limit {
		t.Errorf("expected limit %d, got %d", limit, decLimit)
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	invalidCursors := []string{
		"invalid-base64-!!!",
		"YWJj",                     // base64 of "abc", not valid JSON
		"eyJvZmZzZXQiOiAiYmFkIn0=", // valid base64 JSON but wrong type
	}

	for _, c := range invalidCursors {
		_, _, err := DecodeCursor(c)
		if err == nil {
			t.Errorf("expected error for invalid cursor %q", c)
		}
	}
}
