## 1. MCP Server Skeleton

- [x] 1.1 Add new Go command `backend/cmd/vt-mcp-tmux-orch` with stdio JSON-RPC loop
- [x] 1.2 Implement MCP framing: accept `Content-Length` and fallback line-delimited JSON
- [x] 1.3 Implement `initialize` and `tools/list` responses

## 2. Orchestrator Core (Go)

- [x] 2.1 Add internal package (e.g. `backend/internal/tmuxorch`) with state model + JSON persistence under `.codex/tools/tmux-orch/state/`
- [x] 2.2 Implement `.codex/tools/tmux-orch/` path layout helpers (plans/logs/results)
- [x] 2.3 Implement tmux helpers: has-session/new-session/split/tiled/send-keys/ctrl-c/kill-session
- [x] 2.4 Implement worker file generation (prompt + script) and launch command using `codex exec --json` (analyze uses read-only sandbox)
- [x] 2.5 Implement git worktree ensure under `.worktree-tmux-orch/` for modify execution kind
- [x] 2.6 Implement status refresh from done files + pane/session existence; auto-close session when all terminal

## 3. Tools Implementation

- [x] 3.1 Implement `tmux_orch_doctor`
- [x] 3.2 Implement `tmux_orch_draft` (run_id generation, mode/execution_kind detection, plan generation)
- [x] 3.3 Implement `tmux_orch_revise` (feedback parsing; resize same-task workers; execution kind/policy toggles)
- [x] 3.4 Implement `tmux_orch_run` (create session/panes; ensure worktrees; start workers)
- [x] 3.5 Implement `tmux_orch_control` (stop/inject/restart)
- [x] 3.6 Implement `tmux_orch_status` (refresh + structured response)
- [x] 3.7 Implement `tmux_orch_close` (kill session if present)

## 4. Documentation + Wiring

- [x] 4.1 Update `PROJECT_STRUCTURE.md` to add the new MCP tool and artifact directory
- [x] 4.2 Add a short MCP Settings JSON example for registering `tmux-orch` server (command points to built binary)

## 5. Tests / Verification

- [x] 5.1 Add unit tests for MCP framing parser/serializer
- [x] 5.2 Add unit tests for state persistence and path layout
- [x] 5.3 Add a basic integration test (or manual script) to exercise tools: draft -> run -> status -> close (skip tmux/codex execution when tools missing)
- [x] 5.4 Run `go test ./...` under `backend/` and record results
