import re

files_to_patch = ["internal/engine/treesitter.go", "internal/engine/anonymizer.go"]

type_map = {
    "function_definition": "ast_types.TypePythonFunctionDefinition",
    "function_declaration": "ast_types.TypeGoFunctionDeclaration",
    "method_definition": "ast_types.TypeTypescriptMethodDefinition",
    "method_declaration": "ast_types.TypeGoMethodDeclaration",
    "function_item": "ast_types.TypeRustFunctionItem",
    "class_definition": "ast_types.TypePythonClassDefinition",
    "class_declaration": "ast_types.TypeTypescriptClassDeclaration",
    "call_expression": "ast_types.TypeGoCallExpression", # Or Ts/Rust/Py Call
    "try_statement": "ast_types.TypeTypescriptTryStatement",
    "except_clause": "ast_types.TypePythonExceptClause",
    "catch_clause": "ast_types.TypeTypescriptCatchClause",
    "for_statement": "ast_types.TypeGoForStatement",
    "while_statement": "ast_types.TypePythonWhileStatement",
    "if_statement": "ast_types.TypeGoIfStatement",
    "go_statement": "ast_types.TypeGoGoStatement",
    "throw_statement": "ast_types.TypeTypescriptThrowStatement", # not generated yet, let's leave as is for ones not generated
    "import_statement": "ast_types.TypePythonImportStatement",
    "import_from_statement": "ast_types.TypePythonImportFromStatement",
    "match_statement": "ast_types.TypePythonMatchStatement",
}

for filepath in files_to_patch:
    with open(filepath, 'r') as f:
        content = f.read()

    if "ast_types" not in content and filepath == "internal/engine/treesitter.go":
        content = content.replace('"github.com/odvcencio/gotreesitter"', '"github.com/odvcencio/gotreesitter"\n\t"github.com/Rogercode97/scouter/internal/engine/ast_types"')
    if "ast_types" not in content and filepath == "internal/engine/anonymizer.go":
        content = content.replace('"github.com/odvcencio/gotreesitter"', '"github.com/odvcencio/gotreesitter"\n\t"github.com/Rogercode97/scouter/internal/engine/ast_types"')

    # Replace specific known kinds
    for k, v in type_map.items():
        # Only replace exact string matches
        content = re.sub(rf'"{k}"', v, content)
        
    with open(filepath, 'w') as f:
        f.write(content)
