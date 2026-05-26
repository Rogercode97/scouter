package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// ChronosSnapshot represents a point-in-time structural map of a file.
type ChronosSnapshot struct {
	ID        string
	FilePath  string
	Symbols   map[string]types.ASTPointer // Key: Symbol Name
	Timestamp time.Time
}

// ChronosDiff represents the structural differences found after an edit.
type ChronosDiff struct {
	MissingSymbols []string // Symbols that existed before but were deleted
	MangledSymbols []string // Symbols whose structural hash changed (broken syntax/logic)
	AddedSymbols   []string // New symbols added
	Unchanged      int      // Number of symbols that remained perfectly intact
}

type ChronosEngine struct{}

func NewChronosEngine() *ChronosEngine {
	return &ChronosEngine{}
}

// TakeSnapshot reads a file and captures its structural AST hashes.
func (e *ChronosEngine) TakeSnapshot(ctx context.Context, filePath string) (*ChronosSnapshot, error) {
	absPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return nil, err
	}

	ptrIter, _, err := StreamWithTreeSitter(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("chronos failed to stream AST: %w", err)
	}

	snapshot := &ChronosSnapshot{
		FilePath:  absPath,
		Symbols:   make(map[string]types.ASTPointer),
		Timestamp: time.Now(),
	}

	for ptr := range ptrIter {
		// Only track high-level structural nodes
		if ptr.Type == "function" || ptr.Type == "method" || ptr.Type == "class" || ptr.Type == "interface" {
			snapshot.Symbols[ptr.Name] = ptr
		}
	}

	// Generate a deterministic ID based on the file and timestamp
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", absPath, snapshot.Timestamp.UnixNano())))
	snapshot.ID = hex.EncodeToString(hash[:16])

	return snapshot, nil
}

// CompareSnapshot verifies a file against a previously taken snapshot.
func (e *ChronosEngine) CompareSnapshot(ctx context.Context, snapshot *ChronosSnapshot, currentFilePath string) (*ChronosDiff, error) {
	absPath, err := utils.ValidatePath(currentFilePath)
	if err != nil {
		return nil, err
	}

	ptrIter, _, err := StreamWithTreeSitter(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("chronos failed to stream AST for comparison: %w", err)
	}

	currentSymbols := make(map[string]types.ASTPointer)
	for ptr := range ptrIter {
		if ptr.Type == "function" || ptr.Type == "method" || ptr.Type == "class" || ptr.Type == "interface" {
			currentSymbols[ptr.Name] = ptr
		}
	}

	diff := &ChronosDiff{
		MissingSymbols: make([]string, 0),
		MangledSymbols: make([]string, 0),
		AddedSymbols:   make([]string, 0),
	}

	// 1. Check for Missing and Mangled symbols
	for name, oldSym := range snapshot.Symbols {
		newSym, exists := currentSymbols[name]
		if !exists {
			diff.MissingSymbols = append(diff.MissingSymbols, name)
		} else {
			// Check both structural (signature) and logic (body) hashes
			sigChanged := oldSym.StructuralHash != newSym.StructuralHash
			logicChanged := oldSym.LogicHash != "" && newSym.LogicHash != "" && oldSym.LogicHash != newSym.LogicHash
			if sigChanged || logicChanged {
				diff.MangledSymbols = append(diff.MangledSymbols, name)
			} else {
				diff.Unchanged++
			}
		}
	}

	// 2. Check for newly Added symbols
	for name := range currentSymbols {
		if _, exists := snapshot.Symbols[name]; !exists {
			diff.AddedSymbols = append(diff.AddedSymbols, name)
		}
	}

	return diff, nil
}
