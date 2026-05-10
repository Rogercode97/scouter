package engine

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func TestAnonymizer(t *testing.T) {
	lang := tree_sitter.NewLanguage(tree_sitter_go.Language())
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)

	code1 := []byte(`func Add(a, b int) int { return a + b }`)
	code2 := []byte(`func Sum(x, y int) int { return x + y }`)

	tree1 := parser.Parse(code1, nil)
	defer tree1.Close()
	tree2 := parser.Parse(code2, nil)
	defer tree2.Close()

	root1 := tree1.RootNode()
	root2 := tree2.RootNode()

	sExpr1 := AnonymizeNode(root1, code1)
	sExpr2 := AnonymizeNode(root2, code2)

	if sExpr1 != sExpr2 {
		t.Errorf("Anonymized S-Expressions should be identical.\n1: %s\n2: %s", sExpr1, sExpr2)
	}
}

func TestStructuralHash(t *testing.T) {
	lang := tree_sitter.NewLanguage(tree_sitter_go.Language())
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)

	code1 := []byte(`func Process(data string) { fmt.Println(data) }`)
	code2 := []byte(`func Handle(input string) { fmt.Println(input) }`)

	tree1 := parser.Parse(code1, nil)
	defer tree1.Close()
	tree2 := parser.Parse(code2, nil)
	defer tree2.Close()

	hash1 := GetStructuralHash(tree1.RootNode(), code1)
	hash2 := GetStructuralHash(tree2.RootNode(), code2)

	if hash1 != hash2 {
		t.Errorf("Structural hashes should be identical for identical logic.\n1: %s\n2: %s", hash1, hash2)
	}

	// Different logic should have different hash
	code3 := []byte(`func Process(data string) { return }`)
	tree3 := parser.Parse(code3, nil)
	defer tree3.Close()
	hash3 := GetStructuralHash(tree3.RootNode(), code3)

	if hash1 == hash3 {
		t.Errorf("Different logic should yield different hashes.\n1: %s\n3: %s", hash1, hash3)
	}
}
