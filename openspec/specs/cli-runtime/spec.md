# cli-runtime Specification

## Purpose

CLI runtime 定义 `vibe-tree` 默认 AI 执行路径的统一 contract：主 chat / workflow / orchestration 通过外部 CLI agent 执行，SDK 仅保留为 helper-only 能力。

## Requirements

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

### Requirement: SDK helper execution MUST remain opt-in and isolated
SDK execution MUST remain available only for helper-class operations such as thinking translation, LLM connectivity testing, and other explicitly approved single-purpose utility tasks.

The system MUST NOT auto-select helper SDK execution for default chat, workflow, or orchestration runs.

#### Scenario: Thinking translation remains a helper SDK call
- **WHEN** a chat turn has thinking translation enabled
- **THEN** the reasoning translation subtask may invoke the helper SDK configuration
- **AND** the parent chat turn still runs through the CLI runtime

### Requirement: CLI wrappers MUST write session.json for chat turns
CLI wrappers used by chat turns MUST write a `session.json` artifact whenever the underlying CLI exposes a resumable session/thread identifier.

#### Scenario: Wrapper writes session.json
- **WHEN** a chat turn completes and the CLI exposes a session/thread id
- **THEN** the wrapper writes `session.json` containing the tool id and session id

### Requirement: Chat wrappers MUST prefer native resume when session id exists
When a chat session already has a CLI-native session/thread id, wrappers MUST prefer the tool's native resume mechanism and accept only the current turn input from the application layer.

#### Scenario: Codex wrapper resumes by stored session id
- **WHEN** a chat turn provides `VIBE_TREE_RESUME_SESSION_ID` to the Codex wrapper
- **THEN** the wrapper invokes `codex exec resume` for that turn

