## ADDED Requirements

### Requirement: Workflow planning and AI worker runs MUST use CLI runtime by default
When a workflow run requires AI planning or AI worker execution, the system MUST resolve a CLI-capable expert by default and start that work through the shared CLI runtime.

Existing workflow lifecycle APIs, state transitions, execution logging, and cancellation surfaces MUST remain compatible.

Helper SDK experts MUST NOT be auto-selected for workflow planning or AI worker execution.

#### Scenario: Workflow master planning starts through CLI runtime
- **WHEN** a user starts a workflow that uses an AI master expert
- **THEN** the master execution is launched through the CLI runtime by default
- **AND** the workflow still enters the existing planning lifecycle

#### Scenario: Workflow AI worker runs through CLI runtime
- **WHEN** the scheduler starts an AI worker node in a running workflow
- **THEN** the resolved worker execution uses the CLI runtime by default
- **AND** the resulting execution remains observable through the existing execution log and cancel APIs
