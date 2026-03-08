## 1. Config and persistence

- [x] 1.1 Add MCP and Skill config models plus normalization/helpers
- [x] 1.2 Add chat session persistence for selected MCP ids and migration coverage
- [x] 1.3 Add focused config/store tests for defaults and round-trip behavior

## 2. Backend settings and runtime

- [x] 2.1 Add MCP settings API with per-tool default bindings
- [x] 2.2 Add Skill settings API with discovery output and per-tool bindings
- [x] 2.3 Inject Codex thread `config.mcp_servers` and skill allowlist into runtime
- [x] 2.4 Extend chat session create/update flows to initialize and save MCP session selection

## 3. Frontend settings and chat UX

- [x] 3.1 Add MCP settings tab and daemon client types
- [x] 3.2 Add Skill settings tab and daemon client types
- [x] 3.3 Add new-session/current-session MCP selection UI to chat page
- [x] 3.4 Keep Skill defaults tool-enabled without requiring per-conversation selection UI

## 4. Validation and docs

- [x] 4.1 Update PROJECT_STRUCTURE.md for new settings/runtime responsibilities
- [x] 4.2 Run focused backend and frontend validation commands
- [x] 4.3 Archive the completed OpenSpec change
