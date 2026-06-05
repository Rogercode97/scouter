package filter

import (
	"strings"
	"testing"
)

func TestAstGrepResiliency(t *testing.T) {
	// Mock input NDJSON with some malformed lines
	ndjson := `{"text": "match1", "range": {"start": {"line": 1, "column": 1}, "end": {"line": 1, "column": 10}}, "lines": "match1"}
invalid json line
{"text": "match2", "range": {"start": {"line": 2, "column": 1}, "end": {"line": 2, "column": 10}}, "lines": "match2"}
`

	a := &AstGrepFilter{}
	reader := strings.NewReader(ndjson)

	// We need to expose the parsing logic to test it without running a real 'sg' command
	matches, err := a.parseJSONStream(reader)
	if err != nil {
		t.Fatalf("parseJSONStream failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}

	expected := []string{"match1", "match2"}
	for i, m := range expected {
		if matches[i] != m {
			t.Errorf("expected match %d to be %q, got %q", i, m, matches[i])
		}
	}
}
