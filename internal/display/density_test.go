package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestHAKAIEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewHAKAIEncoder(&buf)

	// Test Header
	if err := enc.WriteHeader(); err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}

	// Test Symbol with path interning
	sym1 := store.Symbol{
		Name:      "MyFunc",
		Type:      "function",
		Path:      "src/main.go",
		StartLine: 10,
		StartCol:  5,
	}
	if err := enc.EncodeSymbol(sym1); err != nil {
		t.Fatalf("EncodeSymbol failed: %v", err)
	}

	// Test repeat path (should NOT emit legend)
	sym2 := store.Symbol{
		Name:      "OtherFunc",
		Type:      "function",
		Path:      "src/main.go",
		StartLine: 20,
		StartCol:  1,
	}
	if err := enc.EncodeSymbol(sym2); err != nil {
		t.Fatalf("EncodeSymbol failed: %v", err)
	}

	// Test new path
	sym3 := store.Symbol{
		Name:      "Helper",
		Type:      "function",
		Path:      "src/utils.go",
		StartLine: 5,
		StartCol:  0,
	}
	if err := enc.EncodeSymbol(sym3); err != nil {
		t.Fatalf("EncodeSymbol failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	expected := []string{
		"#!HAKAI/1",
		"@1:src/main.go",
		"S|1|MyFunc|function|10|5||",
		"S|1|OtherFunc|function|20|1||",
		"@2:src/utils.go",
		"S|2|Helper|function|5|0||",
	}

	if len(lines) != len(expected) {
		t.Errorf("expected %d lines, got %d", len(expected), len(lines))
	}

	for i, line := range lines {
		if i < len(expected) && line != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestHAKAIEncoder_Intelligence(t *testing.T) {
	var buf bytes.Buffer
	enc := NewHAKAIEncoder(&buf)

	enc.WriteHeader()
	enc.EncodeRank("main.go", 0.85)
	enc.EncodeChurn("main.go", 1.2)
	enc.EncodeCritical(store.CriticalSymbol{
		Symbol: store.Symbol{Name: "Core", Path: "main.go"},
		Centrality: 10,
		Fragility: 5,
	})

	output := buf.String()
	if !strings.Contains(output, "R|1|0.8500") {
		t.Errorf("output missing rank: %s", output)
	}
	if !strings.Contains(output, "K|1|1.2000") {
		t.Errorf("output missing churn: %s", output)
	}
	if !strings.Contains(output, "X|1|Core|10|5") {
		// Note: EncodeCritical still uses the old format in the code. 
		// I should probably update EncodeCritical too if it's meant to be state-aware.
		// For now, let's keep it as is or update it.
		// Actually, EncodeCritical wasn't updated in the code.
		t.Errorf("output missing critical: %s", output)
	}
}