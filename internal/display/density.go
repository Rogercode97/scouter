package display

import (
	"fmt"
	"io"

	"github.com/Rogercode97/scouter/internal/store"
)

// MUNCHEncoder implements the Multi-Path Unit Compact Hierarchy format.
// It provides a high-density, token-efficient wire format for code symbols and relationships.
type MUNCHEncoder struct {
	w      io.Writer
	paths  map[string]int
	nextID int
}

// NewMUNCHEncoder creates a new MUNCH encoder writing to w.
func NewMUNCHEncoder(w io.Writer) *MUNCHEncoder {
	return &MUNCHEncoder{
		w:      w,
		paths:  make(map[string]int),
		nextID: 1,
	}
}

// WriteHeader writes the MUNCH protocol header.
func (e *MUNCHEncoder) WriteHeader() error {
	_, err := fmt.Fprintln(e.w, "#MUNCH/1")
	return err
}

// getPathID returns the interned ID for a path, emitting a legend entry if it's the first encounter.
func (e *MUNCHEncoder) getPathID(path string) (int, error) {
	if id, ok := e.paths[path]; ok {
		return id, nil
	}
	id := e.nextID
	e.paths[path] = id
	e.nextID++
	_, err := fmt.Fprintf(e.w, "@%d:%s\n", id, path)
	return id, err
}

// EncodeSymbol encodes a symbol row.
// Format: S|PathID|Name|Type|StartLine|StartCol
func (e *MUNCHEncoder) EncodeSymbol(s store.Symbol) error {
	id, err := e.getPathID(s.Path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "S|%d|%s|%s|%d|%d\n", id, s.Name, s.Type, s.StartLine, s.StartCol)
	return err
}

// EncodeCall encodes a caller relationship row.
// Format: C|PathID|CallerName
func (e *MUNCHEncoder) EncodeCall(c store.Call) error {
	id, err := e.getPathID(c.Path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "C|%d|%s\n", id, c.CallerName)
	return err
}

// EncodeRank encodes a PageRank metric row.
// Format: R|PathID|Rank
func (e *MUNCHEncoder) EncodeRank(path string, rank float64) error {
	id, err := e.getPathID(path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "R|%d|%.4f\n", id, rank)
	return err
}

// EncodeChurn encodes a churn metric row.
// Format: K|PathID|Churn
func (e *MUNCHEncoder) EncodeChurn(path string, churn float64) error {
	id, err := e.getPathID(path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "K|%d|%.4f\n", id, churn)
	return err
}

// EncodeCritical encodes a critical symbol row.
// Format: X|PathID|Name|Centrality|Fragility
func (e *MUNCHEncoder) EncodeCritical(s store.CriticalSymbol) error {
	id, err := e.getPathID(s.Path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "X|%d|%s|%d|%d\n", id, s.Name, s.Centrality, s.Fragility)
	return err
}