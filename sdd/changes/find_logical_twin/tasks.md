<tasks>
  <task id="1.0" type="milestone" name="AST Anonymization Engine" depends_on="">
    <action>Implement the AST visitor to mask identifiers and literals into an S-Expression.</action>
    <verify>go test ./internal/engine -run TestAnonymizer</verify>
    <commit>feat(engine): implement AST anonymizer for structural twin</commit>
  </task>

  <task id="1.1" model="pro" depends_on="1.0" status="completed">
    <action>Update internal/store/store.go to include structural_hash in the symbols table schema and the SaveSymbol / SearchSymbols logic.</action>
    <verify>go test ./internal/store/...</verify>
    <commit>feat(store): add structural_hash to symbols table</commit>
  </task>

  <task id="2.0" type="milestone" name="MCP Integration" depends_on="1.1">
    <action>Register `find_logical_twin` tool in the MCP handlers.</action>
    <verify>go test ./internal/mcp -run TestFindLogicalTwinHandler</verify>
    <commit>feat(mcp): register find_logical_twin tool</commit>
  </task>
</tasks>