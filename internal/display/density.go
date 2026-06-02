package display

import (
	"fmt"
	"io"

	"github.com/Rogercode97/scouter/internal/store"
)

// HAKAIEncoder implements the High-density Adaptive Knowledge Atlas Integration format.
// It provides a high-density, token-efficient wire format for code symbols and relationships.
type HAKAIEncoder struct {
	w      io.Writer
	paths  map[string]int
	nextID int
	State  SovereignState
}

// NewHAKAIEncoder creates a new HAKAI encoder writing to w.
func NewHAKAIEncoder(w io.Writer) *HAKAIEncoder {
	return &HAKAIEncoder{
		w:      w,
		paths:  make(map[string]int),
		nextID: 1,
		State:  HOT,
	}
}

// WriteHeader writes the HAKAI protocol header.
func (e *HAKAIEncoder) WriteHeader() error {
	_, err := fmt.Fprintln(e.w, "#!HAKAI/1")
	return err
}

// getPathID returns the interned ID for a path, emitting a legend entry if it's the first encounter.
func (e *HAKAIEncoder) getPathID(path string) (int, error) {
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
// Format: S|PathID|Name|Type|StartLine|StartCol|Signature|Doc
func (e *HAKAIEncoder) EncodeSymbol(s store.Symbol) error {
	id, err := e.getPathID(s.Path)
	if err != nil {
		return err
	}
	if e.State == HOT {
		_, err = fmt.Fprintf(e.w, "S|%d|%s|%s|%d|%d|%s|%s\n", id, s.Name, s.Type, s.StartLine, s.StartCol, s.Signature, s.Doc)
	} else {
		_, err = fmt.Fprintf(e.w, "S|%d|%s|%s|%d|%d\n", id, s.Name, s.Type, s.StartLine, s.StartCol)
	}
	return err
}

// EncodeCall encodes a caller relationship row.
// Format: C|PathID|CallerName
func (e *HAKAIEncoder) EncodeCall(c store.Call) error {
	id, err := e.getPathID(c.Path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "C|%d|%s\n", id, c.CallerName)
	return err
}

// EncodeRank encodes a Pagerank metric row.
// Format: R|PathID|Rank
func (e *HAKAIEncoder) EncodeRank(path string, rank float64) error {
	id, err := e.getPathID(path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "R|%d|%.4f\n", id, rank)
	return err
}

// EncodeChurn encodes a churn metric row.
// Format: K|PathID|Churn
func (e *HAKAIEncoder) EncodeChurn(path string, churn float64) error {
	id, err := e.getPathID(path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "K|%d|%.4f\n", id, churn)
	return err
}

// EncodeCritical encodes a critical symbol row.
// Format: X|PathID|Name|Centrality|Fragility
func (e *HAKAIEncoder) EncodeCritical(s store.CriticalSymbol) error {
	id, err := e.getPathID(s.Path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "X|%d|%s|%d|%d\n", id, s.Name, s.Centrality, s.Fragility)
	return err
}