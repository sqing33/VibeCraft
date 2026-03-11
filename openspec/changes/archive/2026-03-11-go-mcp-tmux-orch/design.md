## Context

We already have a working tmux-based parallel orchestrator implemented as a Codex skill script (`.codex/skills/tmux-codex-orchestrator/scripts/tmux_orch.py`). However, the Codex/Claude CLI will not reliably choose to use it unless the capability is exposed as a callable tool (MCP). We also want to avoid Python/Node runtime dependencies for Linux distribution.

The repository already supports configuring MCP servers via settings and injecting them into Codex runtime (`mcp_servers`), so adding a new MCP server is primarily about providing a local stdio MCP process that implements `tools/list` and `tools/call`.

Constraints:
- Linux-first; tmux is required.
- Stdio MCP transport; no listening TCP port.
- Minimal initial surface area: core orchestration lifecycle without synthesis/reporting.
- Run artifacts should live under a tool-owned directory (`.codex/tools/tmux-orch/`) instead of the skill directory.

## Goals / Non-Goals

**Goals:**
- Provide a Go MCP server binary that exposes tmux orchestration as tools usable in Codex/Claude sessions.
- Implement orchestrator core in Go (tmux + git worktree + worker file generation + status refresh).
- Persist state/logs/results under `.codex/tools/tmux-orch/` for predictable artifact management.
- Keep behavior compatible with the existing script's core semantics (execution_kind, direct vs review_first, worker statuses).

**Non-Goals:**
- Windows support.
- Full parity with the existing Python orchestrator (no synthesize/report/inspect in v1).
- Integrating orchestration into vibe-tree daemon HTTP APIs or UI.
- Auto-selecting the tool without any prompting (selection is still driven by the model + system instructions).

## Decisions

1. **Independent MCP server process (stdio)**
   - Rationale: matches Codex/Claude MCP expectations; avoids coupling to daemon lifecycle and avoids TCP ports.
   - Alternative: daemon-hosted MCP or HTTP API; rejected for added complexity and lower relevance to "conversation tools".

2. **Go implementation (no Python/Node runtime)**
   - Rationale: simplifies distribution for Linux (and later packaging); aligns with "no node/python in release images".
   - Alternative: Python MCP wrapper; fastest but keeps Python dependency.

3. **Artifact directory migration**
   - `.codex/tools/tmux-orch/` becomes the new root for plan/state/log/result.
   - Rationale: tool artifacts should not be "owned" by a skill directory; reduces coupling to skill enablement.

4. **MCP framing support**
   - Implement Content-Length framed JSON-RPC, and accept line-delimited JSON as a fallback.
   - Rationale: different clients vary; robustness improves interoperability.

## Risks / Trade-offs

- [MCP framing mismatch] → Support both Content-Length and line-delimited input; keep responses Content-Length.
- [tmux availability or session conflicts] → Provide `doctor`; enforce stable session naming; surface errors as tool failures.
- [Git worktree side-effects] → Keep all worktrees under `.worktree-tmux-orch/`; never delete worker branches/worktrees automatically in v1.
- [Model doesn't call tools] → Provide a small "usage policy" snippet to include in base instructions for relevant experts.

