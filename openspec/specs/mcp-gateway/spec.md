# mcp-gateway Specification

## Purpose
TBD - created by archiving change add-mcp-gateway-daemon. Update Purpose after archive.
## Requirements
### Requirement: Daemon MUST expose a managed MCP gateway endpoint
The daemon MUST expose a single managed MCP gateway endpoint that aggregates all configured MCP servers for local CLI runtimes.

The gateway MUST be served by the daemon process itself and MUST support Streamable HTTP transport on a stable daemon-owned route.
The gateway MUST require daemon-issued authorization credentials for client requests.
The gateway MUST identify the calling chat session from the presented credentials and MUST apply that session's current MCP allowlist when listing or calling tools.

#### Scenario: CLI connects to daemon gateway
- **WHEN** a CLI runtime is configured for MCP access
- **THEN** it connects to the daemon-managed gateway route instead of directly connecting to individual saved MCP servers
- **AND** the gateway authenticates the request before exposing tools

#### Scenario: Session-scoped tool listing is filtered
- **WHEN** a chat session is allowed to use only a subset of saved MCP servers
- **THEN** the gateway `tools/list` response includes tools only from that allowed subset
- **AND** tools from disallowed servers are absent

### Requirement: Gateway MUST manage downstream MCP lifecycle on demand
The gateway MUST manage saved downstream MCP servers by workspace and server id.

For local stdio servers, the gateway MUST be able to start the configured command on demand and reuse the process while it remains active.
For remote servers, the gateway MUST be able to establish and reuse a client connection on demand.
The gateway MUST reclaim idle downstream runtimes after a configurable idle TTL.
The gateway MUST retain enough saved configuration to recreate a downstream runtime after it has been reclaimed.

#### Scenario: Idle server is recreated on next use
- **WHEN** a downstream MCP runtime has been reclaimed after idle TTL expiry
- **AND** a later request needs a tool from that server
- **THEN** the gateway recreates the downstream runtime from saved MCP configuration
- **AND** forwards the tool call after the downstream runtime becomes ready

#### Scenario: Concurrent cold calls share one startup
- **WHEN** multiple requests arrive for the same stopped downstream MCP server
- **THEN** the gateway performs only one startup attempt for that server
- **AND** the waiting requests reuse the same ready runtime or receive the same startup failure

### Requirement: Gateway MUST publish a stable aggregated tool directory
The gateway MUST maintain a stable aggregated tool directory for downstream MCP tools.

The gateway MUST expose public tool names that remain unique across downstream servers.
The gateway MUST map each exposed tool name back to the original downstream server id and downstream tool name when forwarding a call.
The gateway MUST refresh a downstream server's tool metadata after that server starts or reconnects.

#### Scenario: Duplicate downstream tool names stay callable
- **WHEN** two downstream MCP servers expose the same original tool name
- **THEN** the gateway exposes distinct public tool names for each tool
- **AND** each public tool name routes to the correct downstream server at call time

#### Scenario: Gateway refreshes tool metadata after restart
- **WHEN** a downstream MCP server is restarted or reconnected
- **THEN** the gateway refreshes that server's tool metadata
- **AND** later tool listings reflect the refreshed metadata

### Requirement: Gateway MUST expose settings and runtime status
The system MUST expose gateway configuration and runtime status through daemon APIs and the MCP settings UI.

Gateway configuration MUST include at least an enabled flag and idle TTL seconds.
Gateway status MUST include whether the gateway is enabled, whether it is currently reachable, and the current downstream server runtime states and recent errors.

#### Scenario: Settings API returns gateway config
- **WHEN** the client requests MCP settings
- **THEN** the response includes the persisted gateway configuration
- **AND** the UI can render whether the gateway is enabled and what idle TTL is configured

#### Scenario: Status API reports downstream runtime state
- **WHEN** the client requests gateway status
- **THEN** the daemon returns the current downstream runtime states for the active workspaces
- **AND** each reported runtime includes last-used or recent error information when available

