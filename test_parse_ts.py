import tree_sitter_typescript
import tree_sitter
from tree_sitter import Language, Parser

parser = Parser(Language(tree_sitter_typescript.language_typescript()))
tree = parser.parse(b"export async function fetchData(url: string) {}")
print(tree.root_node.sexp())
