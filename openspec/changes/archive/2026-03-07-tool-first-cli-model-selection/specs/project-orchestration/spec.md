## ADDED Requirements

### Requirement: Orchestration primary CLI experts MUST use tool-bound default models
When orchestration agent or synthesis runs resolve to a primary CLI tool expert, the runtime MUST use that tool's configured default model unless a valid override is provided.

#### Scenario: Orchestration agent uses claude default model
- **WHEN** an orchestration agent run resolves to the `claude` tool expert
- **THEN** the execution uses the default model configured for the `claude` CLI tool
