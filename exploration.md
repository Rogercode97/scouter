# Exploration: Documentation Sovereignty

## Go AST Analysis
In Go, comments associated with declarations are available via the `Doc` field of `*ast.FuncDecl` and `*ast.GenDecl`.
By using `parser.ParseComments` in `parser.ParseFile`, these are automatically linked.

Example:
```go
func (fn *ast.FuncDecl) DocText() string {
    if fn.Doc == nil {
        return ""
    }
    return fn.Doc.Text()
}
```

For `GenDecl` (Type, Var, Const), it's similar:
```go
if gd, ok := n.(*ast.GenDecl); ok {
    doc := gd.Doc.Text()
    // Process specs (TypeSpec, ValueSpec)
}
```

## Tree-sitter Analysis
Tree-sitter doesn't automatically link comments to declarations. We must:
1.  Look for `comment` nodes as previous siblings.
2.  Handle docstrings for Python (check if the first node in the `block` is a string).

### Generic approach for comments:
```go
func getPrecedingComments(node tree_sitter.Node, content []byte) string {
    var comments []string
    curr := node.PrevNamedSibling()
    for curr != nil && curr.Type() == "comment" {
        comments = append([]string{curr.Utf8Text(content)}, comments...)
        curr = curr.PrevNamedSibling()
    }
    return strings.Join(comments, "\n")
}
```

## OOM Guard & Formatting
- Truncate at 1000 characters.
- Strip markers (`//`, `/*`, `*/`, `#`, `\"\"\"`).

## Schema Migration
SQLite `symbols` table needs `doc TEXT` column.
Since we use `CREATE TABLE IF NOT EXISTS`, we might need a `MIGRATE` query:
```sql
ALTER TABLE symbols ADD COLUMN doc TEXT;
```
