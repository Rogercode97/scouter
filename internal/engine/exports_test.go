package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/stretchr/testify/assert"
)

func dumpSexp(t *testing.T, ext, content string) {
	var lang *gotreesitter.Language
	switch ext {
	case ".go":
		lang = grammars.GoLanguage()
	case ".py":
		lang = grammars.PythonLanguage()
	case ".rs":
		lang = grammars.RustLanguage()
	case ".ts", ".tsx", ".js", ".jsx":
		lang = grammars.TypescriptLanguage()
	}

	parser := gotreesitter.NewParser(lang)
	tree, _ := parser.Parse([]byte(content))
	fmt.Printf("--- %s SEXP ---\n%s\n", ext, tree.RootNode().SExpr(lang))
}

func TestExportSexp(t *testing.T) {
	dumpSexp(t, ".py", `__all__ = ["foo", "bar"]`)
	dumpSexp(t, ".rs", `pub use crate::module::MyStruct;
pub use std::collections::{HashMap, HashSet};`)
	dumpSexp(t, ".ts", `export * from './foo';
export { foo as bar } from './foo';`)

	// Test ParseExports
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")
	os.WriteFile(pyFile, []byte(`__all__ = ["foo", "bar"]`), 0644)
	
	pyExports, err := ParseExports(context.Background(), pyFile)
	assert.NoError(t, err)
	assert.Len(t, pyExports, 2)
	assert.Equal(t, "foo", pyExports[0].Symbol)
	assert.Equal(t, "bar", pyExports[1].Symbol)
	assert.True(t, pyExports[0].IsAll)

	rsFile := filepath.Join(tmpDir, "test.rs")
	os.WriteFile(rsFile, []byte(`pub use crate::module::MyStruct;
pub use std::collections::{HashMap, HashSet};`), 0644)
	rsExports, err := ParseExports(context.Background(), rsFile)
	assert.NoError(t, err)
	assert.Len(t, rsExports, 3)
	assert.Equal(t, "MyStruct", rsExports[0].Symbol)
	assert.Equal(t, "HashMap", rsExports[1].Symbol)
	assert.Equal(t, "HashSet", rsExports[2].Symbol)

	tsFile := filepath.Join(tmpDir, "test.ts")
	os.WriteFile(tsFile, []byte(`export * from './foo';
export { foo as bar } from './foo';`), 0644)
	tsExports, err := ParseExports(context.Background(), tsFile)
	assert.NoError(t, err)
	assert.Len(t, tsExports, 2)
	assert.Equal(t, "*", tsExports[0].Symbol)
	assert.Equal(t, "./foo", tsExports[0].Source)
	assert.True(t, tsExports[0].IsAll)
	assert.Equal(t, "bar", tsExports[1].Symbol)
	assert.Equal(t, "./foo", tsExports[1].Source)
}
