package types

// StructuralRule encapsulates a structural search pattern and relational constraints.
type StructuralRule struct {
	Pattern string `json:"pattern"`
	Inside  string `json:"inside,omitempty"`
	Has     string `json:"has,omitempty"`
}
