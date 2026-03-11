# mcp-tmux-orchestrator Specification

## Purpose

`mcp-tmux-orchestrator` defines a local MCP server (stdio, no port) and a tmux-based orchestration runtime that enables Codex/Claude to spawn and control parallel Codex workers via `tools/call`.

## Requirements

### Requirement: MCP server MUST expose tmux orchestrator tools over stdio

The system MUST provide a local MCP server that communicates over stdio (no TCP port) and exposes tmux-based parallel orchestration tools so that Codex/Claude CLI can call them via `tools/call`.

The MCP server MUST implement `tools/list` and `tools/call` and MUST provide at least the following tools:
- `tmux_orch_doctor`
- `tmux_orch_draft`
- `tmux_orch_revise`
- `tmux_orch_run`
- `tmux_orch_control`
- `tmux_orch_status`
- `tmux_orch_close`

The MCP server MUST return structured JSON responses for each tool call, including `run_id` and a snapshot of persisted run state when applicable.

#### Scenario: Client lists tools
- **WHEN** a client calls `tools/list`
- **THEN** the server returns a tool list containing the tmux orchestrator tools

#### Scenario: Client calls draft tool
- **WHEN** a client calls `tools/call` for `tmux_orch_draft` with a non-empty goal
- **THEN** the server creates a new run state and returns `run_id` and state snapshot

### Requirement: MCP server input MUST support Content-Length framing and MAY support line-delimited JSON

The MCP server MUST accept Content-Length framed JSON-RPC messages on stdin.

The MCP server MAY also accept a single-line JSON message format as a fallback for clients that do not use Content-Length framing.

#### Scenario: Content-Length framed request
- **WHEN** the client sends a JSON-RPC request framed with a valid `Content-Length` header
- **THEN** the server parses the message and returns the response framed with `Content-Length`

#### Scenario: Line-delimited request
- **WHEN** the client sends a single-line JSON-RPC request without headers
- **THEN** the server parses the message and returns the response

### Requirement: Orchestrator artifacts MUST persist under a tool-owned directory

The orchestrator MUST store run artifacts under `.codex/tools/tmux-orch/` with stable subdirectories:
- `state/<run_id>.json`
- `plans/ORCH_PLAN.md` (or an equivalent plan file location under the tool directory)
- `logs/<run_id>/...`
- `results/<run_id>/...`

The server MUST NOT write run artifacts into `.codex/skills/tmux-codex-orchestrator/` for new runs.

#### Scenario: Draft creates state under tool directory
- **WHEN** `tmux_orch_draft` succeeds for a run
- **THEN** the corresponding state file exists under `.codex/tools/tmux-orch/state/<run_id>.json`

#### Scenario: Run creates worker artifacts under tool directory
- **WHEN** `tmux_orch_run` launches workers
- **THEN** worker prompt/script/log/done/message artifacts are created under `.codex/tools/tmux-orch/` subdirectories

### Requirement: Orchestrator MUST support analyze and modify execution kinds

The orchestrator MUST support two execution kinds:
- `modify`: workers may create git worktrees/branches and make changes
- `analyze`: workers MUST be constrained to read-only behavior and the orchestrator MUST invoke codex with read-only sandbox flags

#### Scenario: Analyze-only run uses read-only sandbox
- **WHEN** a run has `execution_kind=analyze`
- **THEN** the worker launch command uses Codex read-only sandbox flags

