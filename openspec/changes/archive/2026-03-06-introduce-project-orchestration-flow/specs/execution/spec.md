## ADDED Requirements

### Requirement: Execution events MUST carry orchestration correlation when applicable
If an execution belongs to an orchestration agent run, the system MUST associate that execution with `orchestration_id`, `round_id`, and `agent_run_id`.

The `execution.started`, `node.log`, and `execution.exited` WebSocket events for such an execution MUST include those orchestration correlation identifiers in addition to `execution_id`.

Legacy workflow executions MUST remain compatible with existing workflow/node consumers.

#### Scenario: Agent run execution emits orchestration-aware events
- **WHEN** an execution starts for an orchestration agent run
- **THEN** its lifecycle events include `execution_id`, `orchestration_id`, `round_id`, and `agent_run_id`
- **AND** clients can correlate logs and terminal state back to the agent run

#### Scenario: Legacy workflow execution remains compatible
- **WHEN** an execution starts for a legacy workflow node
- **THEN** existing workflow/node correlation remains available
- **AND** clients that do not use orchestration identifiers continue to function

### Requirement: Execution log and cancel surfaces MUST remain reusable for agent runs
An orchestration agent run that has an active or prior execution MUST expose the execution identifier needed to reuse the existing execution log tail and execution cancel surfaces.

#### Scenario: Agent run detail fetches execution log tail
- **WHEN** a client selects an agent run with a current or previous execution
- **THEN** the agent run detail provides the associated `execution_id`
- **AND** the client can reuse the existing execution log tail API for that execution

#### Scenario: Canceling an active agent run uses execution cancel semantics
- **WHEN** an orchestration control flow requests cancellation for an active agent run
- **THEN** the underlying execution is canceled through the shared execution cancellation mechanism
- **AND** the execution reaches a terminal canceled state using the same lifecycle semantics as other executions
