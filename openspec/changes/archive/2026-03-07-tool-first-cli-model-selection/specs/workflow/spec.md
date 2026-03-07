## ADDED Requirements

### Requirement: Workflow primary CLI experts MUST use tool-bound default models
When workflow planning or worker execution resolves to a primary CLI tool expert, the runtime MUST use that tool's configured default model unless a valid per-run override is provided.

#### Scenario: Workflow master uses codex default model
- **WHEN** workflow planning runs through the `codex` tool expert
- **THEN** the execution uses the default model configured for the `codex` CLI tool
