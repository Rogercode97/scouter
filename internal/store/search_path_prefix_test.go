package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// The indexer writes absolute paths: internal/engine/indexer.go runs every path
// through utils.ValidatePath, which absolutizes it. These fixtures mirror that
// on purpose. TestStoreSearch seeds relative paths, and a fixture in the wrong
// shape cannot catch a path filter that never matches what the product stores.
func seedSymbolAtAbsolutePath(t *testing.T, path string) Store {
	t.Helper()

	s, err := NewStore(t.Context(), filepath.Join(t.TempDir(), "scouter.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	if err := s.SaveFileIndex(t.Context(), &FileIndex{
		Path: path, Mtime: 1, Hash: "hash", AstJson: "{}", Project: "scouter",
	}); err != nil {
		t.Fatalf("SaveFileIndex: %v", err)
	}
	if err := s.SaveSymbol(t.Context(), &Symbol{
		Name: "Convergeable", Type: "func", Path: path,
		StartByte: 0, EndByte: 10, StartLine: 1, StartCol: 1, EndLine: 1,
	}); err != nil {
		t.Fatalf("SaveSymbol: %v", err)
	}
	return s
}

// An empty prefix means "no path restriction", which is how the callers that
// work already use it, including internal/engine/analyzer_test.go.
func TestSearchSymbols_EmptyPrefixFindsAbsolutePaths(t *testing.T) {
	const path = "/tmp/proj/internal/store/store.go"
	s := seedSymbolAtAbsolutePath(t, path)

	got, err := s.SearchSymbols(t.Context(), "Convergeable", "", "", 10, 0)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 symbol, got %d", len(got))
	}
	if got[0].Path != path {
		t.Errorf("path = %q, want %q", got[0].Path, path)
	}
}

// A relative prefix can never match a stored absolute path. Returning zero rows
// for it makes a misconfigured filter indistinguishable from a query with no
// matches, which is how scouter search shipped returning nothing at all while
// printing a confident "No results found".
func TestSearchSymbols_RejectsRelativePathPrefix(t *testing.T) {
	const path = "/tmp/proj/internal/store/store.go"
	s := seedSymbolAtAbsolutePath(t, path)

	_, err := s.SearchSymbols(t.Context(), "Convergeable", "", ".", 10, 0)
	if err == nil {
		t.Fatal(`SearchSymbols with prefix "." returned no error; a prefix that cannot match must fail`)
	}
	if !strings.Contains(err.Error(), `"."`) {
		t.Errorf("error should name the offending prefix, got: %v", err)
	}

	got, err := s.SearchSymbols(t.Context(), "Convergeable", "", "", 10, 0)
	if err != nil || len(got) != 1 {
		t.Errorf("the rejected call disturbed the store: got %d symbols, err %v", len(got), err)
	}
}

func TestSearchHybrid_RejectsRelativePathPrefix(t *testing.T) {
	s := seedSymbolAtAbsolutePath(t, "/tmp/proj/internal/store/store.go")

	if _, err := s.SearchHybrid(t.Context(), "Convergeable", nil, ".", 10); err == nil {
		t.Fatal(`SearchHybrid with prefix "." returned no error`)
	}
}

func TestSearchSymbolsWeighted_RejectsRelativePathPrefix(t *testing.T) {
	s := seedSymbolAtAbsolutePath(t, "/tmp/proj/internal/store/store.go")

	var got error
	for _, err := range s.SearchSymbolsWeighted(t.Context(), "Convergeable", "", ".") {
		if err != nil {
			got = err
		}
	}
	if got == nil {
		t.Fatal(`SearchSymbolsWeighted with prefix "." yielded no error`)
	}
}
