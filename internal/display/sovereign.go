package display

import (
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/Rogercode97/scouter/internal/store"
)

// SovereignState defines the data density tier for context frames.
type SovereignState string

const (
	HOT  SovereignState = "HOT"
	WARM SovereignState = "WARM"
	COLD SovereignState = "COLD"
)

// Default thresholds for state transitions.
const (
	DefaultThresholdWarm = 500
	DefaultThresholdCold = 2000
)

// ProtocolHeader is the versioned prefix for the Sovereign Context Protocol.
const ProtocolHeader = "#!SOV/1"

// SovereignContext defines the interface for state-aware context management.
type SovereignContext interface {
	io.Writer
	SetState(state SovereignState)
	GetState() SovereignState
	EmitSymbol(s store.Symbol) error
	EmitCall(c store.Call) error
	EmitRank(path string, rank float64) error
	EmitChurn(path string, churn float64) error
}

// SovereignWrapper wraps an io.Writer to provide state-aware context generation.
type SovereignWrapper struct {
	mu            sync.Mutex
	inCheck       bool // Reentrancy guard
	Writer        io.Writer
	Encoder       *HAKAIEncoder
	State         SovereignState
	ACCP          *ACCP
	residualBytes int
	coldBuffer    map[string][]store.Symbol
}

// NewSovereignWrapper creates a new wrapper around the provided writer.
func NewSovereignWrapper(w io.Writer) *SovereignWrapper {
	s := &SovereignWrapper{
		Writer:     w,
		State:      HOT,
		coldBuffer: make(map[string][]store.Symbol),
	}
	s.Encoder = NewHAKAIEncoder(lockedWriter{s})
	return s
}

// lockedWriter is a thin wrapper to satisfy io.Writer without re-locking.
type lockedWriter struct {
	s *SovereignWrapper
}

func (lw lockedWriter) Write(p []byte) (int, error) {
	return lw.s.writeLocked(p)
}

// SetACCP attaches an ACCP engine to the wrapper.
func (s *SovereignWrapper) SetACCP(accp *ACCP) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ACCP = accp
}

// TrackTokens updates the ACCP engine with the number of tokens emitted and adjusts the state if necessary.
func (s *SovereignWrapper) TrackTokens(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ACCP == nil {
		return
	}
	s.ACCP.TotalTokens += count
	s.proactiveCheck()
}

// proactiveCheck ensures the wrapper state matches the ACCP's recommended state.
// This is called before emissions to avoid "Transition Lag".
func (s *SovereignWrapper) proactiveCheck() {
	if s.ACCP == nil || s.inCheck {
		return
	}
	s.inCheck = true
	defer func() { s.inCheck = false }()

	newState := s.ACCP.DetermineState(s.ACCP.TotalTokens)
	if newState != s.State {
		s.setStateLocked(newState)
		// Emit state transition record via locked writer to satisfy IO Seal and avoid deadlock.
		fmt.Fprintf(lockedWriter{s}, "#!STATE|%s\n", newState)
	}
}

// Write implements io.Writer by delegating to the underlying writer.
// It updates the internal token count to prevent seal bypasses using a lossless accumulator.
func (s *SovereignWrapper) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(p)
}

func (s *SovereignWrapper) writeLocked(p []byte) (n int, err error) {
	n, err = s.Writer.Write(p)
	if err == nil && s.ACCP != nil {
		s.residualBytes += n
		tokens := s.residualBytes / 4
		s.residualBytes %= 4

		if tokens > 0 {
			s.ACCP.TotalTokens += tokens
			s.proactiveCheck()
		}
	}
	return n, err
}

// SetState updates the current context state and synchronizes the encoder.
func (s *SovereignWrapper) SetState(state SovereignState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setStateLocked(state)
}

func (s *SovereignWrapper) setStateLocked(state SovereignState) {
	s.State = state
	if s.Encoder != nil {
		s.Encoder.State = state
	}
}

// GetState returns the current context state.
func (s *SovereignWrapper) GetState() SovereignState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State
}

// WriteHeader writes the Sovereign protocol header and current state.
func (s *SovereignWrapper) WriteHeader() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := fmt.Fprintf(lockedWriter{s}, "%s|%s\n", ProtocolHeader, s.State)
	return err
}

// Flush emits all buffered COLD hashes.
func (s *SovereignWrapper) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

func (s *SovereignWrapper) flushLocked() error {
	if len(s.coldBuffer) == 0 {
		return nil
	}

	paths := make([]string, 0, len(s.coldBuffer))
	for path := range s.coldBuffer {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		symbols := s.coldBuffer[path]
		hash := ComputeULMENHash(symbols)
		_, err := fmt.Fprintf(lockedWriter{s}, "@COLD:%s:%s\n", path, hash)
		if err != nil {
			return err
		}
		delete(s.coldBuffer, path)
	}
	return nil
}

// EmitSymbol emits a symbol according to the current state.
func (s *SovereignWrapper) EmitSymbol(sym store.Symbol) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proactiveCheck()
	switch s.State {
	case COLD:
		s.coldBuffer[sym.Path] = append(s.coldBuffer[sym.Path], sym)
		return nil
	case WARM, HOT:
		return s.Encoder.EncodeSymbol(sym)
	default:
		return fmt.Errorf("unknown state: %s", s.State)
	}
}

// ComputeULMENHash generates a deterministic SHA-256 hash for a set of symbols.
func ComputeULMENHash(symbols []store.Symbol) string {
	// 1. Sort symbols deterministically to ensure hash stability.
	sorted := make([]store.Symbol, len(symbols))
	copy(sorted, symbols)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		if sorted[i].StartLine != sorted[j].StartLine {
			return sorted[i].StartLine < sorted[j].StartLine
		}
		return sorted[i].StartCol < sorted[j].StartCol
	})

	h := sha256.New()
	for _, sym := range sorted {
		// 2. Use length-prefixed fields for safe serialization (prevent delimiter collisions).
		writePrefixed(h, sym.Path)
		writePrefixed(h, sym.Name)
		writePrefixed(h, sym.Type)
		fmt.Fprintf(h, ":L%d:C%d|", sym.StartLine, sym.StartCol)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// writePrefixed writes a length-prefixed string to the provided writer.
func writePrefixed(w io.Writer, s string) {
	fmt.Fprintf(w, "%d:%s", len(s), s)
}

// ValidateULMENHash checks if a given hash matches the current state of symbols.
func ValidateULMENHash(hash string, symbols []store.Symbol) bool {
        return ComputeULMENHash(symbols) == hash
}

// EmitCall emits a call relationship according to the current state.
func (s *SovereignWrapper) EmitCall(c store.Call) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proactiveCheck()
	switch s.State {
	case COLD:
		// COLD state omits calls or just emits metadata?
		// Spec says "contains only file paths and ULMEN hashes".
		return nil
	case WARM, HOT:
		return s.Encoder.EncodeCall(c)
	default:
		return fmt.Errorf("unknown state: %s", s.State)
	}
}

// EmitRank emits a PageRank metric if the state is not COLD.
func (s *SovereignWrapper) EmitRank(path string, rank float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proactiveCheck()
	if s.State == COLD {
		return nil
	}
	return s.Encoder.EncodeRank(path, rank)
}

// EmitChurn emits a churn metric if the state is not COLD.
func (s *SovereignWrapper) EmitChurn(path string, churn float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proactiveCheck()
	if s.State == COLD {
		return nil
	}
	return s.Encoder.EncodeChurn(path, churn)
}

// ACCP (Adaptive Context Compression Protocol) manages token density and state transitions.
type ACCP struct {
	ThresholdWarm int
	ThresholdCold int
	TotalTokens   int
	Frames        []ContextFrame
}

// ContextFrame represents a unit of context (e.g., a file) in the sliding window.
type ContextFrame struct {
	Path   string
	Tokens int
}

// NewACCP creates a new ACCP engine with the specified token thresholds.
func NewACCP(warm, cold int) *ACCP {
	return &ACCP{
		ThresholdWarm: warm,
		ThresholdCold: cold,
		Frames:        make([]ContextFrame, 0),
	}
}

// DetermineState returns the appropriate SovereignState based on the provided token count.
func (a *ACCP) DetermineState(tokens int) SovereignState {
	if tokens > a.ThresholdCold {
		return COLD
	}
	if tokens > a.ThresholdWarm {
		return WARM
	}
	return HOT
}

// RegisterFrame adds a new frame to the ACCP window and updates the total token count.
func (a *ACCP) RegisterFrame(path string, tokens int) {
	a.Frames = append(a.Frames, ContextFrame{Path: path, Tokens: tokens})
	a.TotalTokens += tokens
}
