package engine

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestAnonymizer(t *testing.T) {
	lang := grammars.GoLanguage()
	parser := gotreesitter.NewParser(lang)

	code1 := []byte(`func Add(a, b int) int { return a + b }`)
	code2 := []byte(`func Sum(x, y int) int { return x + y }`)

	tree1, _ := parser.Parse(code1)
	tree2, _ := parser.Parse(code2)

	root1 := tree1.RootNode()
	root2 := tree2.RootNode()

	sExpr1 := AnonymizeNode(root1, code1, lang)
	sExpr2 := AnonymizeNode(root2, code2, lang)

	if sExpr1 != sExpr2 {
		t.Errorf("Anonymized S-Expressions should be identical.\n1: %s\n2: %s", sExpr1, sExpr2)
	}
}

func TestStructuralHash(t *testing.T) {
	lang := grammars.GoLanguage()
	parser := gotreesitter.NewParser(lang)

	code1 := []byte(`func Process(data string) { fmt.Println(data) }`)
	code2 := []byte(`func Handle(input string) { fmt.Println(input) }`)

	tree1, _ := parser.Parse(code1)
	tree2, _ := parser.Parse(code2)

	hash1 := GetStructuralHash(tree1.RootNode(), code1, lang)
	hash2 := GetStructuralHash(tree2.RootNode(), code2, lang)

	if hash1 != hash2 {
		t.Errorf("Structural hashes should be identical for identical logic.\n1: %s\n2: %s", hash1, hash2)
	}

	// Different logic should have different hash
	code3 := []byte(`func Process(data string) { return }`)
	tree3, _ := parser.Parse(code3)
	hash3 := GetStructuralHash(tree3.RootNode(), code3, lang)

	if hash1 == hash3 {
		t.Errorf("Different logic should yield different hashes.\n1: %s\n3: %s", hash1, hash3)
	}
}
