# Tasks: MCP Server Sovereignty Fix

<task_list>

    <phase name="Foundation: Staging Ledger Enhancements">
        <task id="LDR-001" status="todo" trace="staging-ledger/Scenario: Generate a unified diff">
            <title>Implement Unified Diff in Ledger</title>
            <description>Add a Diff() method to internal/engine/ledger.go that generates a unified diff string from all staged patches.</description>
        </task>
        <task id="LDR-002" status="todo" trace="staging-ledger/Scenario: Persist staged changes">
            <title>Implement Staging Persistence</title>
            <description>Add SaveStaging() and LoadStaging() methods to Ledger for disk persistence in .scouter/staging/.</description>
        </task>
        <task id="LDR-003" status="todo" trace="staging-ledger/Scenario: Clear staging area">
            <title>Add ClearStaging method</title>
            <description>Implement a method to wipe the staging area and delete the persistence file.</description>
        </task>
    </phase>

    <phase name="Core: MCP Protocol Hardening">
        <task id="MCP-001" status="todo" trace="mcp-server/Scenario: Graceful fallback on Sampling failure">
            <title>Sampling Fallback in MCPMessenger</title>
            <description>Modify internal/mcp/messenger.go to catch -32601 errors and return a fallback message.</description>
            <depends_on id="LDR-001" />
        </task>
        <task id="MCP-002" status="todo" trace="static-resources/Scenario: Read Dependency Graph resource">
            <title>Register Dependency Graph Resource</title>
            <description>Expose file:///scouter/graph/dependencies in internal/mcp/resources.go with truncation logic.</description>
        </task>
        <task id="MCP-003" status="todo" trace="static-resources/Scenario: Read MCP Schema resource">
            <title>Register MCP Schema Resource</title>
            <description>Expose file:///scouter/schema/mcp in internal/mcp/resources.go.</description>
        </task>
    </phase>

    <phase name="Integration: Dry-Run Support">
        <task id="INT-001" status="todo" trace="mcp-server/Scenario: Dry-run execution for mutation tools">
            <title>Add DryRun to RippleRefactor</title>
            <description>Update RippleRefactorParams and handleDryRun logic in internal/mcp/handlers.go.</description>
            <depends_on id="MCP-001" />
        </task>
        <task id="INT-002" status="todo" trace="mcp-server/Scenario: Dry-run execution for mutation tools">
            <title>Add DryRun to Evolve</title>
            <description>Update EvolveParams and integrate with Ledger.Stage in internal/mcp/handlers.go.</description>
            <depends_on id="INT-001" />
        </task>
        <task id="INT-003" status="todo" trace="staging-ledger/Scenario: Commit staged changes">
            <title>Expose CommitStaged Tool</title>
            <description>Add a new MCP tool 'scouter_commit' to apply staged changes to disk.</description>
            <depends_on id="LDR-002" />
        </task>
    </phase>

    <phase name="Validation: Testing &amp; Verification">
        <task id="VAL-001" status="todo">
            <title>Unit Tests for Ledger Diff &amp; Persistence</title>
            <description>Create internal/engine/ledger_test.go covering diff and staging logic.</description>
            <depends_on id="LDR-002" />
        </task>
        <task id="VAL-002" status="todo">
            <title>Integration Test: Dry-Run Lifecycle</title>
            <description>Create tests/dry_run_test.go to verify full cycle: stage -> diff -> commit.</description>
            <depends_on id="INT-003" />
        </task>
        <task id="VAL-003" status="todo">
            <title>Verify Resource Accessibility</title>
            <description>Run smoke tests to ensure resources return valid (and truncated) data.</description>
            <depends_on id="MCP-003" />
        </task>
    </phase>

</task_list>
