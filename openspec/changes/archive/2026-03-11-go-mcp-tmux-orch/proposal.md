## Why

Today `tmux-codex-orchestrator` only exists as a Codex skill plus a local script entrypoint. In practice, Codex/Claude CLI will not proactively invoke this orchestration method during a conversation, because it is not exposed as a callable tool. This blocks reliable “LLM-driven parallel work” where the model can decide to fan out work into multiple tmux workers and then control and observe progress.

## What Changes

- Add a Go-based MCP server (stdio, no port) that exposes tmux-based parallel Codex orchestration as callable tools (`draft/run/control/status/close`, plus `doctor/revise`).
- Implement the orchestrator core in Go (Linux-first) so it does not depend on Python or Node runtimes.
- Store orchestrator state/logs/results under a new tools directory: `.codex/tools/tmux-orch/`, instead of the skill directory.
- Update project structure documentation to include the new MCP tool and artifact locations.

## Capabilities

### New Capabilities
- `mcp-tmux-orchestrator`: Provide an MCP server that exposes tmux-based parallel Codex orchestration tools over stdio, and persists run artifacts under `.codex/tools/tmux-orch/`.

### Modified Capabilities
- (none)

## Impact

- New Go command under `backend/cmd/` for running the MCP server binary.
- New Go internal package implementing tmux/git/worktree orchestration behavior.
- New `.codex/tools/tmux-orch/` artifact directory; existing skill artifacts are not removed.
- Users will add an MCP server entry in the MCP settings UI to enable the tool for Codex/Claude sessions.

