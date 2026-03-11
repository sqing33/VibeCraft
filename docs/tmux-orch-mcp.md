# tmux-orch MCP Server

This repository provides a local MCP server that exposes tmux-based parallel Codex orchestration tools over stdio.

## MCP Settings JSON Example

Paste this JSON into the `MCP` settings tab (adjust the `command` to your built binary path):

```json
{
  "mcpServers": {
    "tmux-orch": {
      "command": "/abs/path/to/vt-mcp-tmux-orch",
      "args": [],
      "description": "tmux-based parallel Codex orchestrator (stdio MCP)"
    }
  }
}
```

## Tools

The server provides tools:
- `tmux_orch_doctor`
- `tmux_orch_draft`
- `tmux_orch_revise`
- `tmux_orch_run`
- `tmux_orch_control`
- `tmux_orch_status`
- `tmux_orch_close`

## Artifacts

Runs persist under:
- `.codex/tools/tmux-orch/state/`
- `.codex/tools/tmux-orch/plans/ORCH_PLAN.md`
- `.codex/tools/tmux-orch/logs/`
- `.codex/tools/tmux-orch/results/`

