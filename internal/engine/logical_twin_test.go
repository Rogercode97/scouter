package engine

import (
	"context"
	"os"
	"testing"

	"github.com/Rogercode97/scouter/internal/types"
)

func TestLogicalTwinHashing(t *testing.T) {
	content := `package main

func Add(a, b int) int {
	return a + b
}

func Sum(x, y int) int {
	return x + y
}

func Multiply(a, b int) int {
	return a * b
}
`
	tmpFile, err := os.CreateTemp("", "twin_test*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if err := os.WriteFile(tmpFile.Name(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	itPointers, _, err := StreamSymbols(ctx, tmpFile.Name())
	if err != nil {
		t.Fatalf("StreamSymbols failed: %v", err)
	}

	var symbols []types.ASTPointer
	for sym := range itPointers {
		symbols = append(symbols, sym)
	}

	var addHash, sumHash, mulHash string
	for _, sym := range symbols {
		switch sym.Name {
		case "Add":
			addHash = sym.StructuralHash
		case "Sum":
			sumHash = sym.StructuralHash
		case "Multiply":
			mulHash = sym.StructuralHash
		}
	}

	if addHash == "" {
		t.Error("Add has empty structural hash")
	}
	if sumHash == "" {
		t.Error("Sum has empty structural hash")
	}
	if mulHash == "" {
		t.Error("Multiply has empty structural hash")
	}

	if addHash != sumHash {
		t.Errorf("Add and Sum should have identical structural hashes, got %s and %s", addHash, sumHash)
	}

	if addHash != mulHash {
		t.Errorf("Add and Multiply should have identical structural hashes because bodies are stripped for interface parity, got %s and %s", addHash, mulHash)
	}
}
