# Spec: Context Compaction (v4.5)

## Intent
Allow long-running development sessions to continue indefinitely by summarizing technical state into a persistent anchor file, freeing up the context window without losing progress.

## Requirements

### Requirement: Latent Memory Anchor
Scouter MUST provide a mechanism to summarize the current session and save it to a project-level file.

#### Scenario: Successful Compaction
GIVEN an active development session with multiple changes
WHEN the user executes `scouter_compact_context`
THEN Scouter MUST use MCP Sampling to request a technical summary from the model
AND it MUST save the result to `.scouter/anchor.md`.

### Requirement: Technical Density
The compaction summary SHALL prioritize architectural state, completed tasks, and pending objectives over conversational history.

#### Scenario: Signal Extraction
GIVEN the model provides a summary via sampling
WHEN Scouter saves the anchor file
THEN the content MUST be formatted in high-density Markdown
AND it SHOULD include a timestamp of the compaction.

### Requirement: Anchor Visibility
The existence of a context anchor SHOULD be reported to the user upon successful compaction.
