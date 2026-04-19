package types

type Range struct {
	Start int `json:"start" validate:"gte=0"`
	End   int `json:"end" validate:"gtfield=Start"`
}

type ASTPointer struct {
	Type      string `json:"type" validate:"required,oneof=function class method variable interface"`
	Name      string `json:"name" validate:"required"`
	Doc       string `json:"doc"`
	Range     Range  `json:"range" validate:"required"`
	StartLine int    `json:"start_line" validate:"required,gte=1"`
	EndLine   int    `json:"end_line" validate:"required,gtfield=StartLine"`
	Hash      string `json:"hash" validate:"required,len=64"`
}

type ASTCall struct {
	CallerName string `json:"caller_name" validate:"required"`
	CalleeName string `json:"callee_name" validate:"required"`
	Path       string `json:"path" validate:"required"`
	Line       int    `json:"line" validate:"required,gte=1"`
}

type Dependency struct {
	Name    string `json:"name" validate:"required"`
	Version string `json:"version" validate:"required"`
	Type    string `json:"type" validate:"required,oneof=golang npm"`
	Project string `json:"project" validate:"required"` // Path to the manifest file
	Direct  bool   `json:"direct"`
}
