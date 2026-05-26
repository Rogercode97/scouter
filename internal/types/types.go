package types

type Range struct {
	Start int `json:"start" validate:"gte=0"`
	End   int `json:"end" validate:"gtfield=Start"`
}

type ASTPointer struct {
	Type           string           `json:"type" validate:"required,oneof=function class method variable interface method_spec"`
	Name           string           `json:"name" validate:"required"`
	PackagePath    string           `json:"package_path"`
	ReceiverType   string           `json:"receiver_type"`
	Signature      string           `json:"signature,omitempty"`
	Doc            string           `json:"doc"`
	Range          Range            `json:"range" validate:"required"`
	StartLine      int              `json:"start_line" validate:"required,gte=1"`
	StartCol       int              `json:"start_col" validate:"required,gte=1"`
	EndLine        int              `json:"end_line" validate:"required,gtfield=StartLine"`
	Hash           string           `json:"hash" validate:"required,len=64"`
	StructuralHash string           `json:"structural_hash,omitempty" validate:"omitempty,len=64"`
	Metrics        *SemanticMetrics `json:"metrics,omitempty"`
}

type ASTCall struct {
	CallerName string `json:"caller_name" validate:"required"`
	CalleeName string `json:"callee_name" validate:"required"`
	CalleePath string `json:"callee_path"` // Optional: absolute path to the callee file
	LinkType   string `json:"link_type"`   // call, implements, emits, etc.
	Path       string `json:"path" validate:"required"`
	Line       int    `json:"line" validate:"required,gte=1"`
}

type RiskMetrics struct {
	Centrality         float64 `json:"centrality"`
	BlastRadius        float64 `json:"blast_radius"`
	PublicExport       bool    `json:"public_export"`
	HistoricalBugfixes int     `json:"historical_bugfixes"`
}

type ImpactEntity struct {
	Symbol    string      `json:"symbol"`
	File      string      `json:"file"`
	Distance  int         `json:"distance"`
	RiskScore float64     `json:"risk_score"`
	LinkType  string      `json:"link_type"`
	Metrics   RiskMetrics `json:"metrics"`
}

type ImpactResult struct {
	Target    ImpactEntity   `json:"target"`
	Callers   []ImpactEntity `json:"callers"`
	Mermaid   string         `json:"mermaid"`
	RiskLevel string         `json:"risk_level"` // Low, Medium, High, Critical
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

type MemoryInsight struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Learned       string   `json:"learned"`
	Why           string   `json:"why"`
	LinkedSymbols []string `json:"linked_symbols,omitempty"`
}

type HybridSearchResult struct {
	Symbols  []Symbol        `json:"symbols"`
	Insights []MemoryInsight `json:"insights"`
}

type TestTarget struct {
	Name string `json:"name"`
	File string `json:"file"`
}

type Symbol struct {
	Name           string           `json:"name"`
	Type           string           `json:"type"`
	PackagePath    string           `json:"package_path"`  // Fully qualified package path
	ReceiverType   string           `json:"receiver_type"` // pointer, value, or empty
	Signature      string           `json:"signature,omitempty"`
	Doc            string           `json:"doc"`
	Path           string           `json:"path"`
	StartLine      int              `json:"start_line"`
	EndLine        int              `json:"end_line"`
	LinkedInsights []string         `json:"linked_insights,omitempty"`
	Metrics        *SemanticMetrics `json:"metrics,omitempty"`
}

type SemanticMetrics struct {
	CyclomaticComplexity int  `json:"cyclomatic_complexity"`
	IsAsync              bool `json:"is_async"`
	HasErrorHandling     bool `json:"has_error_handling"`
	HasExceptions        bool `json:"has_exceptions"`
}

type CompactionResult struct {
	AnchorPath string `json:"anchor_path"`
	Timestamp  string `json:"timestamp"`
	Message    string `json:"message"`
}

type HealResult struct {
	Status     string            `json:"status"`
	FixedCode  string            `json:"fixed_code"`
	TestOutput string            `json:"test_output"`
	Metadata   map[string]string `json:"metadata"`
}