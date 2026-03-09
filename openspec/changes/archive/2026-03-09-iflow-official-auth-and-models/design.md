## Overview

iFlow becomes a first-class special-case CLI runtime. The implementation keeps the shared `cli_tools` registry but adds iFlow-only fields and runtime preparation. Generic LLM sources/models remain available for Codex / Claude and SDK helpers, while iFlow uses only official auth and user-maintained iFlow model names.

## Decisions

### 1. Keep iFlow inside `cli_tools`, but stop using generic model/source wiring

We keep the existing tool registry so the third CLI tool still coexists with Codex and Claude, but we do not resolve iFlow models through `llm.models` anymore.

New iFlow-only config fields:

- `iflow_auth_mode`: `browser` | `api_key`
- `iflow_api_key`: persisted secret
- `iflow_base_url`: defaults to `https://apis.iflow.cn/v1`
- `iflow_models`: ordered user-managed model list
- `iflow_default_model`: default selected iFlow model

### 2. Use an app-managed iFlow home

The daemon uses `$XDG_DATA_HOME/vibe-tree/iflow-home` as the iFlow home for:

- browser-login persistence
- runtime reuse across sessions
- isolation from the user’s global CLI state

A minimal `.iflow/settings.json` bootstrap file is created when needed so iFlow can open its auth dialog reliably.

### 3. Browser auth is driven by a PTY session

A dedicated auth manager launches `iflow` under the managed home and reads PTY output.

Flow:

1. start session
2. wait for auth menu
3. auto-select option 1 (`Login with iFlow(recommend)`)
4. parse the OAuth URL from terminal output
5. expose the URL to the frontend
6. accept pasted authorization code and write it back to the PTY
7. detect successful auth from the managed iFlow settings file
8. stop the PTY session

### 4. MCP injection uses iFlow’s native MCP CLI plus per-turn allow-list

For each iFlow run:

- sync effective MCP server definitions into project scope with `iflow mcp add-json ... --scope project`
- pass `--allowed-mcp-server-names` for the session-selected/default-enabled subset

This preserves existing vibe-tree MCP registry semantics while using iFlow’s native MCP loading path.

### 5. Skill injection is prompt-based

The existing skill binding registry is reused. Effective skills are appended to `VIBE_TREE_SYSTEM_PROMPT` in the same style already used for Codex. This keeps one consistent cross-tool skill model without depending on iFlow marketplace installs.

## Backend Changes

- `config.CLIToolConfig` gains iFlow-only fields and normalization.
- `settings/cli-tools` GET/PUT support masked iFlow API keys and browser-auth status.
- new iFlow auth manager + API endpoints.
- chat runtime adds an iFlow pre-run preparation step for auth, home, MCP, and skill instructions.
- `iflow_exec.sh` switches from OpenAI-compatible envs to official iFlow envs.

## Frontend Changes

### CLI Tool Settings

The iFlow card shows:

- auth mode selector
- browser login session controls/status
- auth URL open action
- authorization code submit box
- API key editor with masked saved state
- model list editor + default model selector

### Chat / Repo Library

When the selected CLI tool is iFlow:

- model choices come from `tool.iflow_models`
- default model comes from `tool.iflow_default_model`
- no shared LLM model compatibility filtering is applied

## Validation

- config normalization tests
- settings API roundtrip tests
- iFlow auth parsing / home bootstrap tests
- iFlow chat runtime preparation tests
- targeted Go tests
- frontend build
