import tree_sitter_python
from tree_sitter import Language, Parser

parser = Parser(Language(tree_sitter_python.language()))
tree = parser.parse(b"__all__ = ['foo', 'bar']")
print("Python:", tree.root_node.sexp())
