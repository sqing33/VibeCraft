## ADDED Requirements

### Requirement: Orchestrations MUST be prompt-first
The system MUST allow a user to start an orchestration directly from a natural-language project goal without pre-creating a DAG or manually creating worker entries first.

When a user submits a goal, the system MUST create a persistent `orchestration` record and start a master planning run for that orchestration.

#### Scenario: User starts orchestration from goal input
- **WHEN** a user submits a goal such as “帮我同时做登录页优化和设置页重构” from the orchestration input area
- **THEN** the system creates a new `orchestration`
- **AND** the orchestration enters a planning/running lifecycle without requiring a pre-created workflow or DAG

#### Scenario: Master decides no fan-out is needed
- **WHEN** the master determines the goal can be handled without spawning child agents
- **THEN** the orchestration remains valid as a single-master run
- **AND** the system still records a synthesis result and terminal outcome

### Requirement: Orchestrations MUST support dynamic multi-round fan-out
The master MUST be able to create `round` records dynamically during execution. Each round MAY contain one or more sibling `agent_run` records chosen at runtime.

All agent runs within the same round MUST be independently trackable and eligible for parallel execution, subject to configured concurrency limits.

#### Scenario: Master creates multiple sibling agents in one round
- **WHEN** the master decides to split work into two parallel tasks
- **THEN** the system creates one `round`
- **AND** the round contains two sibling `agent_run` records with distinct roles/objectives

#### Scenario: Later round is created after prior results
- **WHEN** a round finishes and synthesis concludes more work is needed
- **THEN** the system creates a subsequent `round`
- **AND** the next round is derived from previous round outputs rather than from a precomputed static DAG

### Requirement: Each round MUST end with synthesis before the next round begins
After all non-terminal agent runs in a round reach a terminal state, the system MUST execute a `synthesis_step` for that round.

The synthesis result MUST summarize the round outputs and choose a next action from at least `complete`, `continue`, or `needs_retry`.

The system MUST NOT start the next round until the current round's synthesis step is completed.

#### Scenario: Synthesis runs after all agents finish
- **WHEN** every `agent_run` in round 1 reaches a terminal state
- **THEN** the system creates and completes a `synthesis_step` for round 1
- **AND** the orchestration does not start round 2 before that synthesis step is available

#### Scenario: Synthesis decides to continue
- **WHEN** a synthesis step concludes that additional work is required
- **THEN** the orchestration remains active
- **AND** the next round is created only after the synthesis decision is persisted

### Requirement: Orchestration detail MUST persist rounds, agent runs, and synthesis summaries
The system MUST persist and return orchestration detail with stable identifiers and ordered history for:
- the orchestration itself
- each round
- each agent run
- each synthesis step

Each `agent_run` record MUST include at least role, task goal, status, output summary, whether code was modified, and a reference to its current or last execution when applicable.

#### Scenario: Orchestration detail returns full round history
- **WHEN** a client requests orchestration detail after multiple rounds have completed
- **THEN** the response includes the orchestration metadata and ordered round history
- **AND** each round includes its child agent runs and synthesis step

#### Scenario: Orchestration survives daemon restart
- **WHEN** an orchestration exists and the daemon restarts
- **THEN** previously completed rounds, agent runs, and synthesis summaries remain queryable
- **AND** non-terminal records are recoverable according to persisted state

### Requirement: Orchestrations MUST support cancel, retry, and continue controls
The system MUST support canceling an orchestration, retrying a failed or canceled `agent_run`, and continuing an orchestration after synthesis or recoverable interruption.

Canceling an orchestration MUST stop active executions for that orchestration and transition pending work to a canceled terminal state.

Retrying an `agent_run` MUST preserve prior execution history and create a new execution attempt.

#### Scenario: Cancel a running orchestration
- **WHEN** a user cancels an orchestration with active agent runs
- **THEN** the system cancels all active executions for that orchestration
- **AND** pending agent runs do not continue into later rounds

#### Scenario: Retry a failed agent run
- **WHEN** a failed `agent_run` is retried
- **THEN** the system keeps the previous attempt history
- **AND** creates a new execution attempt associated with that same agent run

#### Scenario: Continue after synthesis
- **WHEN** a synthesis step leaves the orchestration in a continue-able non-terminal state
- **THEN** the system allows the orchestration to proceed into the next planning/execution step
- **AND** the transition is recorded in persistent state
