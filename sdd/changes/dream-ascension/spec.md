## Specification: Dream Ascension: Passive Engram Capture

### Requirements

#### REQ-1: Automated Extraction Hook
- The MCP Server MUST detect the end of a tool-execution sequence.
- It MUST trigger the `PassiveDistill` service asynchronously (non-blocking).
- Extraction MUST only occur if the turn is considered "High Impact" (e.g., file writes, git operations, or > 1000 tokens of context processed).

#### REQ-2: Forked Context Distillation
- The `MCPDistiller` MUST receive the session transcript (messages).
- It MUST use a specialized prompt to extract:
  - **ADRs**: Architectural Decisions and Tradeoffs.
  - **Bugfixes**: Root Cause Analysis and verified fixes.
  - **Patterns**: New conventions or repeated structural choices.
- The output MUST be a structured JSON object.

#### REQ-3: Engram Persistence
- Each extracted item MUST be converted into an `engram.Observation`.
- Observations MUST be saved with the type `architecture`, `bugfix`, or `pattern`.
- Duplicate memories (identical content for the same project) MUST be ignored to prevent database bloat.

#### REQ-4: Mutual Exclusion Guard
- If the user (via the agent) has already called a manual save tool (like `mem_save`) during the current turn, the passive extraction MUST be skipped.

### Technical Constraints
- **Concurrency**: Distillation MUST run in a separate goroutine.
- **Error Handling**: Failures in background distillation MUST be logged silently and MUST NOT crash the main MCP server or delay the response to the user.
- **Context Hygiene**: The distilled memory MUST NOT be injected back into the current conversation (it is for *future* sessions).

### User Stories
- **As a Developer**, I want Scouter to remember that I decided to use Hexagonal Architecture for the new module without me having to manually document it.
- **As a Developer**, I want a bug fix I just applied to be remembered in future sessions so I can ask "Why did we change this function last week?".
