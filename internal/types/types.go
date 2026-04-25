package types

type Range struct {
	Start int `json:"start" validate:"gte=0"`
	End   int `json:"end" validate:"gtfield=Start"`
}

type ASTPointer struct {
	Type      string `json:"type" validate:"required,oneof=function class method variable interface method_spec"`
	Name      string `json:"name" validate:"required"`
	Signature string `json:"signature,omitempty"`
	Doc       string `json:"doc"`
	Range     Range  `json:"range" validate:"required"`
	StartLine int    `json:"start_line" validate:"required,gte=1"`
	EndLine   int    `json:"end_line" validate:"required,gtfield=StartLine"`
	Hash      string `json:"hash" validate:"required,len=64"`
}

type ASTCall struct {
	CallerName string `json:"caller_name" validate:"required"`
	CalleeName string `json:"callee_name" validate:"required"`
	CalleePath string `json:"callee_path"` // Optional: absolute path to the callee file
	LinkType   string `json:"link_type"`   // call, implements, emits, etc.
	Path       string `json:"path" validate:"required"`
	Line       int    `json:"line" validate:"required,gte=1"`
}

type ImpactResult struct {
	Symbol   string `json:"symbol"`
	File     string `json:"file"`
	Distance int    `json:"distance"`
	LinkType string `json:"link_type"`
}

type Dependency struct {
	Name    string `json:"name" validate:"required"`
	Version string `json:"version" validate:"required"`
	Type    string `json:"type" validate:"required,oneof=golang npm"`
	Project string `json:"project" validate:"required"` // Path to the manifest file
	Direct  bool   `json:"direct"`
}

type TestResult struct {
	TestName     string `json:"test_name" validate:"required"`
	Status       string `json:"status" validate:"required,oneof=pass fail skip"`
	ErrorMessage string `json:"error_message,omitempty"`
	StackTrace   string `json:"stack_trace,omitempty"`
	TargetSymbol string `json:"target_symbol,omitempty"`
	DurationMS   int64  `json:"duration_ms" validate:"gte=0"`
	Project      string `json:"project,omitempty"`
}

type TestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action" validate:"required,oneof=run pause cont pass fail skip output bench"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed" validate:"gte=0"`
}