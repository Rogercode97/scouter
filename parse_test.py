import tree_sitter_rust
import tree_sitter_typescript
import tree_sitter
from tree_sitter import Language, Parser

parser = Parser(Language(tree_sitter_rust.language()))
tree = parser.parse(b"pub fn my_func() {}")
print("Rust:")
print(tree.root_node.sexp())

parser = Parser(Language(tree_sitter_typescript.language_typescript()))
tree = parser.parse(b"export function myFunc() {}")
print("TypeScript:")
print(tree.root_node.sexp())
