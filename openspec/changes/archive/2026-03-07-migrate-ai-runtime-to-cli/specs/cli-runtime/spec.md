## ADDED Requirements

### Requirement: CLI runtime MUST be the default AI execution path
The system MUST execute the following primary AI surfaces through CLI runtime by default:

- chat turns
- workflow master planning runs
- workflow AI worker runs
- project orchestration master planning runs
- project orchestration agent runs
- project orchestration synthesis runs

The system MUST NOT route those primary AI surfaces through helper SDK execution unless the request explicitly targets a helper-only operation.

#### Scenario: Chat turn resolves to CLI runtime
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` for a normal chat session
- **THEN** the system resolves a CLI-capable expert by default
- **AND** the assistant output is generated from the CLI runtime instead of a direct provider SDK call

#### Scenario: Orchestration planning resolves to CLI runtime
- **WHEN** a user creates an orchestration from a natural-language goal
- **THEN** the master planning run starts through CLI runtime by default
- **AND** later agent runs and synthesis steps also use CLI runtime unless they are explicitly non-AI or helper-only

### Requirement: CLI runtime MUST expose a standard artifact contract
Every CLI AI run MUST write a stable artifact directory containing machine-readable completion metadata.

The minimum required contract MUST include:

- `summary.json`
- `artifacts.json`

Optional outputs MAY include:

- `final_message.md`
- `tool_calls.jsonl`
- `patch.diff`
- `session.json`

`summary.json` MUST include `status`, `summary`, `modified_code`, `next_action`, and `key_files`.

#### Scenario: Completed CLI run persists summary artifacts
- **WHEN** a CLI AI run exits successfully
- **THEN** the runtime writes `summary.json` and `artifacts.json` into the run artifact directory
- **AND** the owning chat/workflow/orchestration record stores or derives references to that artifact directory

#### Scenario: Analysis-only CLI run reports no code modification
- **WHEN** a CLI AI run finishes without editing files
- **THEN** `summary.json` sets `modified_code` to `false`
- **AND** `artifacts.json` still describes the analysis outputs produced by the run

### Requirement: Planner surfaces MUST exchange structured outputs through the CLI contract
When workflow planning or orchestration planning/synthesis requires machine-readable output, the CLI runtime MUST emit that structured payload through the artifact contract so the daemon can validate it before applying side effects.

#### Scenario: Workflow master returns structured DAG output
- **WHEN** a workflow master planning run completes
- **THEN** the CLI artifact directory contains the structured planning payload needed by the daemon
- **AND** the daemon validates that payload before creating downstream workflow nodes

#### Scenario: Orchestration synthesis returns structured next action
- **WHEN** an orchestration synthesis run completes
- **THEN** the CLI artifact directory contains the structured synthesis decision payload
- **AND** the daemon validates the payload before creating the next round or finalizing the orchestration

### Requirement: SDK helper execution MUST remain opt-in and isolated
SDK execution MUST remain available only for helper-class operations such as thinking translation, LLM connectivity testing, and other explicitly approved single-purpose utility tasks.

The system MUST NOT auto-select helper SDK execution for default chat, workflow, or orchestration runs.

#### Scenario: Thinking translation remains a helper SDK call
- **WHEN** a chat turn has thinking translation enabled
- **THEN** the reasoning translation subtask may invoke the helper SDK configuration
- **AND** the parent chat turn still runs through the CLI runtime

#### Scenario: LLM test remains outside the CLI main path
- **WHEN** client calls `POST /api/v1/settings/llm/test`
- **THEN** the daemon performs a direct helper SDK call
- **AND** it does not create a normal CLI chat, workflow, or orchestration execution
