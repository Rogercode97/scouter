package types

type Range struct {
	Start int `json:"start" validate:"gte=0"`
	End   int `json:"end" validate:"gtfield=Start"`
}

type ASTPointer struct {
	Type      string `json:"type" validate:"required,oneof=function class method variable interface"`
	Name      string `json:"name" validate:"required"`
	Range     Range  `json:"range" validate:"required"`
	StartLine int    `json:"start_line" validate:"required,gte=1"`
	EndLine   int    `json:"end_line" validate:"required,gtfield=StartLine"`
	Hash      string `json:"hash" validate:"required,len=64"`
}
