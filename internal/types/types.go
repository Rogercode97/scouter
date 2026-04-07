package types

type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type ASTPointer struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Range     Range  `json:"range"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Hash      string `json:"hash"`
}
