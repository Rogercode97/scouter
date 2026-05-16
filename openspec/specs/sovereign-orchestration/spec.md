# Delta Spec: MCP Sovereignty Fix (Sovereign Orchestration)

## 1. Requirements (What MUST be true)
- `CompactContextSystemPrompt` MUST output standard Engram format (`**What**`, `**Why**`, `**Where**`, `**Learned**`).
- The Judge system MUST evaluate its findings against injected historical insights (ADRs/patterns).
- The Healer (TruthEngine) MUST inject historical bugfixes into the Healer prompt via memory lookup before generating a fix.
- Handlers (`handleJudge`, `handleCompactContext`) MUST fetch Engram context and inject it into LLM prompts.
- Injected context MUST be bound by token safety limits (e.g., hard cap on injected history tokens or count) to avoid context window explosion.

### Requirement: Validated External Command Execution

The system MUST NOT execute external binaries that are not present in the `allowedBinaries` allow-list. All external calls SHALL pass through `utils.SafeCommand` to ensure context-awareness and argument sanitization.

#### Scenario: Block Forbidden Binary
- GIVEN a call to `utils.SafeCommand` with name \"rm\"
- WHEN the command is initialized
- THEN it MUST return an error \"forbidden binary: rm\"

#### Scenario: Allow Permitted Binary
- GIVEN a call to `utils.SafeCommand` with name \"git\" and arguments [\"status\"]
- WHEN the command is initialized
- THEN it MUST return a valid `*exec.Cmd` object.

#### Scenario: Sanitize Dangerous Arguments
- GIVEN a call to `utils.SafeCommand` with binary \"go\" and arguments [\"test\", \"./...; rm -rf /\"]
- WHEN the command is initialized
- THEN it MUST return an error \"dangerous characters in argument: ./...; rm -rf /\"

## 2. Acceptance Scenarios

### Scenario 1: Session Compaction Persisting to Engram
**Given** the context compaction loop is triggered
**When** `CompactContextSystemPrompt` completes its summarization
**Then** the output MUST strictly adhere to the `**What**`, `**Why**`, `**Where**`, `**Learned**` format
**And** the result MUST be formatted suitably for direct Engram saving without further parsing.

### Scenario 2: Healer Context Enrichment Requirements
**Given** `TruthEngine.Fix` is invoked to heal a bug
**When** preparing the prompt for the Healer
**Then** a memory lookup for relevant historical bugfixes MUST be executed
**And** the top retrieved results MUST be appended to the Healer prompt
**And** the Healer MUST consider these past patterns before generating a fix.

### Scenario 3: Judge Evaluation Against Historical ADRs
**Given** the Judge is evaluating a decision or output
**When** building the `JudgeSystemPrompt`
**Then** relevant historical architectural decisions (ADRs) MUST be retrieved and injected
**And** the Judge MUST mandate checking these historical insights
**And** the final judgment MUST not conflict with injected ADRs unless explicitly justified.

### Scenario 4: Token Safety Limits for Injected Context
**Given** historical context (bugfixes or ADRs) is being fetched for injection
**When** appending this context to any LLM prompt (Healer, Judge, or Compaction)
**Then** a strict token or character safety limit MUST be enforced
**And** any context exceeding this limit MUST be truncated or discarded (prioritizing the most relevant/recent)
**And** the core prompt instructions MUST remain intact and uncompromised.
