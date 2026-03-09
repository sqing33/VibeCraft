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

### Requirement: Chat wrappers MUST emit normalized or parseable streaming events
CLI wrappers used for chat MUST expose a stream that the daemon can parse incrementally for assistant deltas, session updates, and final completion.

For Codex-backed chat turns, the system MAY satisfy this requirement via the official app-server JSON-RPC event stream instead of a shell wrapper, provided that the daemon still exposes normalized `chat.turn.*` events and preserves the artifact contract.

#### Scenario: Wrapper emits assistant delta events
- **WHEN** the underlying CLI emits incremental assistant text
- **THEN** the daemon can relay assistant deltas before turn completion

#### Scenario: Codex chat uses app-server event stream
- **WHEN** a Codex-backed chat turn starts through the app-server transport
- **THEN** the daemon consumes JSON-RPC notifications such as `item/agentMessage/delta` and `item/reasoning/*Delta`
- **AND** relays normalized chat events before turn completion

#### Scenario: Codex chat preserves artifact contract
- **WHEN** a Codex-backed chat turn completes through app-server
- **THEN** the system still writes chat runtime artifacts including `session.json` and `final_message.md`
- **AND** the stored session reference remains reusable by later turns

### Requirement: iFlow runtime MUST use official authentication only
The `iflow` runtime MUST NOT depend on the generic OpenAI-compatible LLM source/model settings.

The runtime MUST support exactly these official iFlow auth inputs:
- browser-auth state persisted in the managed iFlow home
- explicit official iFlow API key from the CLI tool settings card

#### Scenario: iFlow API key auth injects official env
- **WHEN** a chat or repo analysis request selects `iflow` and the tool is configured for `api_key`
- **THEN** the runtime injects official iFlow auth env values
- **AND** it does not inject OpenAI-compatible auth env values

#### Scenario: iFlow browser auth reuses managed home
- **WHEN** a chat or repo analysis request selects `iflow` and the tool is configured for `browser`
- **THEN** the runtime uses the daemon-managed iFlow home for auth reuse
- **AND** it relies on the persisted official browser login state

### Requirement: iFlow runtime MUST support per-turn MCP and skill injection
The `iflow` runtime MUST inject the effective MCP and skill selection derived from vibe-tree settings and the chat session.

#### Scenario: iFlow turn receives effective MCP allow-list
- **WHEN** an iFlow chat turn runs with selected/default MCP server ids
- **THEN** the runtime syncs those MCP definitions into project scope
- **AND** passes only the effective subset through `--allowed-mcp-server-names`

#### Scenario: iFlow turn receives effective skills
- **WHEN** an iFlow chat turn runs with enabled skill bindings
- **THEN** the runtime appends effective skill instructions to the system prompt before launching the CLI

### Requirement: IFLOW wrapper MUST implement the standard CLI artifact contract
The `iflow` wrapper MUST write `summary.json` and `artifacts.json` for every completed run.

When the underlying CLI exposes a resumable session identifier, the wrapper MUST also write `session.json`.
When stdout produces a final assistant response, the wrapper MUST persist it as `final_message.md`.

#### Scenario: IFLOW non-interactive run writes session and final message artifacts
- **WHEN** the `iflow` wrapper completes a non-interactive run with `--output-file`
- **THEN** the wrapper writes `summary.json` and `artifacts.json`
- **AND** it writes `session.json` from the returned execution info when `session-id` is present
- **AND** it writes `final_message.md` from the assistant output

### Requirement: Chat runtime MUST support session-scoped warm Codex app-server reuse
For Codex-backed chat sessions, the daemon MUST support keeping a session-scoped app-server runtime alive across multiple turns within the same daemon lifecycle.

The daemon MUST evict or close warm runtimes when they become idle for too long, when the owning chat session is archived, or when the daemon shuts down.

#### Scenario: Idle runtime is evicted after timeout
- **WHEN** a warm Codex chat runtime remains unused past the configured idle timeout
- **THEN** the daemon closes the underlying app-server process
- **AND** removes the runtime from the in-memory pool

#### Scenario: Archiving a chat session releases its warm runtime
- **WHEN** the client archives a chat session that currently owns a warm Codex runtime
- **THEN** the daemon closes that runtime
- **AND** later follow-up turns require a fresh runtime or a cold `thread/resume`

#### Scenario: Daemon shutdown closes warm runtimes
- **WHEN** the daemon process begins shutdown
- **THEN** all warm Codex chat runtimes are closed before process exit

### Requirement: Codex chat runtime MUST inject only session-selected MCP servers
When a chat turn runs through the Codex app-server transport, the system MUST derive the effective MCP server set from the chat session and selected CLI tool, then pass only that set through the thread request `config` overrides.

When the chat session has no explicit MCP selection yet, the system MUST fall back to the MCP ids that are default-enabled for the selected CLI tool.
The effective MCP candidate set MUST come from the saved MCP registry and MUST NOT depend on a separate tool-level enabled binding.

#### Scenario: Thread start injects selected MCPs
- **WHEN** a new Codex-backed chat session has two selected MCP ids
- **THEN** `thread/start` includes only those two MCP servers in `config.mcp_servers`

#### Scenario: Thread resume preserves selected MCPs
- **WHEN** a Codex-backed chat session resumes an existing thread
- **THEN** `thread/resume` includes the same effective `config.mcp_servers` selection for that session

### Requirement: Codex chat runtime MUST inject effective skill guidance
When a chat turn runs through the Codex app-server transport, the system MUST append an effective skill allowlist to the thread base instructions.

The effective skill set MUST be the currently discovered skill catalog, intersected with the expert `enabled_skills` list when the expert declares one.
The injected guidance MUST include each skill id, a short description when available, its path, and instructions to read `SKILL.md` on demand instead of assuming its contents.
The runtime MUST NOT require tool-level skill binding configuration for a discovered skill to be injected.

#### Scenario: Discovered skill is injected by default
- **WHEN** a skill is discovered from the configured project or user skill roots
- **AND** the active expert does not exclude it
- **THEN** the thread base instructions include that skill in the injected allowlist block

#### Scenario: Expert restriction narrows effective skills
- **WHEN** the active expert declares `enabled_skills` containing only one of several discovered skills
- **THEN** the injected skill allowlist contains only that intersected skill set
