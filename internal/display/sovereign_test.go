package display

import (
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

func TestSovereignConstants(t *testing.T) {
	// Verify SovereignState constants exist and are correct
	if HOT != "HOT" {
		t.Errorf("expected HOT to be 'HOT', got %s", HOT)
	}
	if WARM != "WARM" {
		t.Errorf("expected WARM to be 'WARM', got %s", WARM)
	}
	if COLD != "COLD" {
		t.Errorf("expected COLD to be 'COLD', got %s", COLD)
	}

	// Verify protocol constant
	expectedProtocol := "#!SOV/1"
	if ProtocolHeader != expectedProtocol {
		t.Errorf("expected ProtocolHeader to be '%s', got %s", expectedProtocol, ProtocolHeader)
	}
}

func TestSovereignWrapperStructure(t *testing.T) {
	// Verify SovereignWrapper struct and SovereignContext interface
	var _ SovereignContext = (*SovereignWrapper)(nil)

	wrapper := &SovereignWrapper{
		State: HOT,
	}

	if wrapper.State != HOT {
		t.Errorf("expected wrapper state to be HOT, got %s", wrapper.State)
	}
}

func TestSovereignWrapperStateManagement(t *testing.T) {
	wrapper := NewSovereignWrapper(nil)

	// Test default state
	if wrapper.GetState() != HOT {
		t.Errorf("expected initial state to be HOT, got %s", wrapper.GetState())
	}

	// Test transition to WARM
	wrapper.SetState(WARM)
	if wrapper.GetState() != WARM {
		t.Errorf("expected state to be WARM, got %s", wrapper.GetState())
	}

	// Test transition to COLD
	wrapper.SetState(COLD)
	if wrapper.GetState() != COLD {
		t.Errorf("expected state to be COLD, got %s", wrapper.GetState())
	}
}

func TestSovereignWrapperWriteHeader(t *testing.T) {
	writer := &mockWriter{}
	wrapper := NewSovereignWrapper(writer)

	// Test HOT header
	wrapper.SetState(HOT)
	err := wrapper.WriteHeader()
	if err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}
	expectedHOT := "#!SOV/1|HOT\n"
	if string(writer.data) != expectedHOT {
		t.Errorf("expected HOT header:\n%s\ngot:\n%s", expectedHOT, string(writer.data))
	}

	// Reset writer for COLD header
	writer.data = nil
	wrapper.SetState(COLD)
	err = wrapper.WriteHeader()
	if err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}
	expectedCOLD := "#!SOV/1|COLD\n"
	if string(writer.data) != expectedCOLD {
		t.Errorf("expected COLD header:\n%s\ngot:\n%s", expectedCOLD, string(writer.data))
	}
}

func TestSovereignWrapperEmissions(t *testing.T) {
	writer := &mockWriter{}
	wrapper := NewSovereignWrapper(writer)

	s := store.Symbol{Path: "main.go", Name: "Main", Type: "func", StartLine: 1, StartCol: 1}

	// Test HOT emission
	wrapper.SetState(HOT)
	err := wrapper.EmitSymbol(s)
	if err != nil {
		t.Fatalf("EmitSymbol failed: %v", err)
	}
	// Expected output: HAKAI symbol row with empty Signature and Doc
	expectedHOT := "@1:main.go\nS|1|Main|func|1|1||\n"
	if string(writer.data) != expectedHOT {
		t.Errorf("expected HOT output:\n%s\ngot:\n%s", expectedHOT, string(writer.data))
	}

	// Reset writer for WARM test
	writer.data = nil
	wrapper.SetState(WARM)
	err = wrapper.EmitSymbol(s)
	if err != nil {
		t.Fatalf("EmitSymbol failed: %v", err)
	}
	// Expected output: HAKAI symbol row pruned (no Signature/Doc)
	expectedWARM := "S|1|Main|func|1|1\n"
	if string(writer.data) != expectedWARM {
		t.Errorf("expected WARM output:\n%s\ngot:\n%s", expectedWARM, string(writer.data))
	}

	// Reset writer for COLD test
	writer.data = nil
	wrapper.SetState(COLD)
	err = wrapper.EmitSymbol(s)
	if err != nil {
		t.Fatalf("EmitSymbol failed: %v", err)
	}

	// In COLD state, output is only produced after Flush()
	if len(writer.data) != 0 {
		t.Errorf("expected no output before Flush in COLD state, got %q", string(writer.data))
	}

	err = wrapper.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Expected output: @COLD:main.go:<hash>
	expectedPrefix := "@COLD:main.go:"
	if len(writer.data) < len(expectedPrefix) || string(writer.data)[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected COLD output to start with %s, got %q", expectedPrefix, string(writer.data))
	}
}

func TestSovereignWrapperMetrics(t *testing.T) {
	writer := &mockWriter{}
	wrapper := NewSovereignWrapper(writer)

	// Test HOT metrics
	wrapper.SetState(HOT)
	wrapper.EmitRank("main.go", 0.5)
	expectedHOT := "@1:main.go\nR|1|0.5000\n"
	if string(writer.data) != expectedHOT {
		t.Errorf("expected HOT rank output:\n%s\ngot:\n%s", expectedHOT, string(writer.data))
	}

	// Test COLD metrics suppression
	writer.data = nil
	wrapper.SetState(COLD)
	wrapper.EmitRank("main.go", 0.5)
	if len(writer.data) != 0 {
		t.Errorf("expected COLD rank output to be empty, got %q", string(writer.data))
	}
}

func TestULMENHashingAndValidation(t *testing.T) {

	symbols := []store.Symbol{
		{Path: "main.go", Name: "Main", Type: "func", StartLine: 1, StartCol: 1},
		{Path: "main.go", Name: "Helper", Type: "func", StartLine: 10, StartCol: 1},
	}

	// Test deterministic hashing
	hash1 := ComputeULMENHash(symbols)
	hash2 := ComputeULMENHash(symbols)

	if hash1 != hash2 {
		t.Errorf("expected hashes to be identical for same input, got %s and %s", hash1, hash2)
	}

	// Test validation
	if !ValidateULMENHash(hash1, symbols) {
		t.Errorf("expected hash %s to be valid for symbols", hash1)
	}

	// Test staleness (modified symbols)
	staleSymbols := []store.Symbol{
		{Path: "main.go", Name: "Main", Type: "func", StartLine: 1, StartCol: 1},
		{Path: "main.go", Name: "Helper", Type: "func", StartLine: 11, StartCol: 1}, // Changed line
	}

	if ValidateULMENHash(hash1, staleSymbols) {
		t.Errorf("expected hash %s to be invalid for modified symbols", hash1)
	}

	// Test determinism across different orderings
	symbolsInverted := []store.Symbol{
		{Path: "main.go", Name: "Helper", Type: "func", StartLine: 10, StartCol: 1},
		{Path: "main.go", Name: "Main", Type: "func", StartLine: 1, StartCol: 1},
	}

	hash3 := ComputeULMENHash(symbolsInverted)
	if hash1 != hash3 {
		t.Errorf("expected hashes to be identical for different input orderings, got %s and %s", hash1, hash3)
	}
}

func TestACCPThresholds(t *testing.T) {
	accp := NewACCP(100, 200)

	// Test HOT
	if state := accp.DetermineState(50); state != HOT {
		t.Errorf("expected state HOT for 50 tokens, got %s", state)
	}

	// Test WARM
	if state := accp.DetermineState(150); state != WARM {
		t.Errorf("expected state WARM for 150 tokens, got %s", state)
	}

	// Test COLD
	if state := accp.DetermineState(250); state != COLD {
		t.Errorf("expected state COLD for 250 tokens, got %s", state)
	}
}

func TestACCPWindowManagement(t *testing.T) {
	accp := NewACCP(100, 200)

	// Add a frame
	accp.RegisterFrame("file1.go", 60)
	if accp.TotalTokens != 60 {
		t.Errorf("expected TotalTokens to be 60, got %d", accp.TotalTokens)
	}

	// Add another frame that pushes to WARM
	accp.RegisterFrame("file2.go", 60)
	if accp.TotalTokens != 120 {
		t.Errorf("expected TotalTokens to be 120, got %d", accp.TotalTokens)
	}

	// Check that a new wrapper created now would be WARM
	if state := accp.DetermineState(accp.TotalTokens); state != WARM {
		t.Errorf("expected state WARM for 120 total tokens, got %s", state)
	}
}

type mockWriter struct {
	data []byte
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func TestProactiveACCP(t *testing.T) {
	writer := &mockWriter{}
	wrapper := NewSovereignWrapper(writer)
	// Thresholds: WARM=10, COLD=100
	accp := NewACCP(10, 100)
	wrapper.SetACCP(accp)

	// current tokens: 9. One more symbol will likely push us to WARM.
	accp.TotalTokens = 9
	wrapper.State = HOT

	s := store.Symbol{Path: "main.go", Name: "Main", Type: "func", StartLine: 1, StartCol: 1}

	// We expect EmitSymbol to detect we are near threshold and transition BEFORE emitting the full symbol.
	// Actually, if we want to be truly proactive, we should check the current tokens.
	// If current is 9, and threshold is 10, it's still HOT.
	// But if we are at 10, it should be WARM.
	accp.TotalTokens = 11
	// wrapper.State is still HOT (lag)

	err := wrapper.EmitSymbol(s)
	if err != nil {
		t.Fatalf("EmitSymbol failed: %v", err)
	}

	// In proactive mode, EmitSymbol should have called DetermineState and updated itself.
	if wrapper.State != WARM {
		t.Errorf("expected state to be WARM before emission, got %s", wrapper.State)
	}
}

func TestSealBypass(t *testing.T) {
	writer := &mockWriter{}
	wrapper := NewSovereignWrapper(writer)
	accp := NewACCP(10, 20)
	wrapper.SetACCP(accp)

	// Writing directly to the wrapper should update tokens
	p := []byte("12345678") // 8 bytes = 2 tokens
	wrapper.Write(p)

	if accp.TotalTokens != 2 {
		t.Errorf("expected 2 tokens after writing 8 bytes, got %d", accp.TotalTokens)
	}

	// Writing enough to cross threshold
	wrapper.Write([]byte("12345678123456781234567812345678")) // 32 bytes = 8 tokens. Total = 10.
	// Next write should trigger transition
	wrapper.Write([]byte("1234")) // 4 bytes = 1 token. Total = 11.

	if wrapper.State != WARM {
		t.Errorf("expected state WARM after writing 11 tokens, got %s", wrapper.State)
	}
}

func TestAutomaticEncoderTracking(t *testing.T) {
	writer := &mockWriter{}
	wrapper := NewSovereignWrapper(writer)
	accp := NewACCP(10, 20)
	wrapper.SetACCP(accp)

	s := store.Symbol{Path: "main.go", Name: "Main", Type: "func", StartLine: 1, StartCol: 1}

	// EmitSymbol uses Encoder.EncodeSymbol (if HOT/WARM)
	// which now writes to wrapper, which tracks tokens.
	wrapper.SetState(HOT)
	wrapper.EmitSymbol(s)

	if accp.TotalTokens == 0 {
		t.Errorf("expected tokens to be tracked automatically, got 0")
	}
}
