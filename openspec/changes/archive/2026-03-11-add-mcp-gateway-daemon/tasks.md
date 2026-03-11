## 1. Gateway foundation

- [x] 1.1 Add persisted `mcp_gateway` config, normalization, and settings API fields
- [x] 1.2 Implement daemon MCP gateway module with auth, downstream registry, tool routing, and idle TTL reaping
- [x] 1.3 Mount gateway runtime routes and status API into the daemon server lifecycle

## 2. Runtime integration

- [x] 2.1 Generate session-scoped gateway credentials from chat turns and invalidate incompatible warm Codex runtimes
- [x] 2.2 Switch Codex runtime injection to managed gateway-first behavior with direct-MCP fallback
- [x] 2.3 Switch Claude, OpenCode, and iFlow managed runtime configs/wrappers to inject only the gateway connection

## 3. UI and validation

- [x] 3.1 Extend MCP settings UI and client types for gateway config and runtime status
- [x] 3.2 Add focused backend and runtime tests for gateway config, lifecycle, and session selection updates
- [x] 3.3 Update `PROJECT_STRUCTURE.md` for the new gateway module and settings/runtime responsibilities
