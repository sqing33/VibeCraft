## MODIFIED Requirements

### Requirement: Codex chat runtime MUST inject only session-selected MCP servers
When a chat turn runs through the Codex app-server transport, the system MUST derive the effective MCP server set from the chat session and selected CLI tool.

When the managed MCP gateway is enabled, the runtime MUST pass a single gateway MCP entry through the thread request `config` overrides instead of directly embedding each saved downstream MCP server.
That gateway entry MUST carry daemon-managed connection information and credentials that authorize only the session's current effective MCP selection.

When the chat session has no explicit MCP selection yet, the system MUST fall back to the MCP ids that are default-enabled for the selected CLI tool.
When the managed MCP gateway is disabled, the runtime MAY continue to inject the effective saved MCP entries directly as a backward-compatible fallback.
The effective MCP candidate set MUST come from the saved MCP registry and MUST NOT depend on a separate tool-level enabled binding.

#### Scenario: Thread start injects gateway for selected MCPs
- **WHEN** a new Codex-backed chat session has two selected MCP ids
- **AND** the managed MCP gateway is enabled
- **THEN** `thread/start` includes one managed gateway entry in `config.mcp_servers`
- **AND** that gateway entry is authorized for only those two selected MCP servers

#### Scenario: Gateway-disabled fallback injects direct servers
- **WHEN** the managed MCP gateway is disabled
- **AND** a Codex-backed chat session has selected MCP ids
- **THEN** the runtime injects only those selected saved MCP server entries directly in `config.mcp_servers`

### Requirement: CLI runtimes MUST use application-managed config roots
CLI runtimes MUST apply source-specific configuration through application-managed config roots or per-run config files stored outside the project workspace and outside the user's default global CLI config paths.

Managed config state MUST live under the application's data directory.
The runtime MUST NOT directly overwrite project-local `.codex`, `.claude`, `.iflow`, or user-global default config files as part of normal execution.
When the managed MCP gateway is enabled, each supported CLI runtime MUST materialize only the managed gateway MCP connection into its runtime-specific config shape.
The runtime MUST NOT require the user to separately copy the saved MCP registry into each CLI tool's own config file.

#### Scenario: Codex runtime uses managed CODEX_HOME
- **WHEN** the system launches a Codex CLI turn with a selected runtime model binding
- **THEN** it materializes a managed Codex config root under the application data directory
- **AND** launches Codex with `CODEX_HOME` pointing to that managed root

#### Scenario: Claude runtime uses managed settings file
- **WHEN** the system launches a Claude CLI turn with a selected runtime model binding
- **THEN** it materializes a managed Claude settings file under the application data directory
- **AND** launches Claude with `--settings <managed-file>`

#### Scenario: OpenCode runtime uses managed XDG config root
- **WHEN** the system launches an OpenCode CLI turn with a selected runtime model binding
- **THEN** it materializes a managed OpenCode config root under the application data directory
- **AND** launches OpenCode with `XDG_CONFIG_HOME` pointing to that managed root

#### Scenario: Claude runtime receives gateway MCP only
- **WHEN** the managed MCP gateway is enabled for a Claude chat turn
- **THEN** the managed Claude settings file contains only the gateway MCP connection entry
- **AND** it does not need the full saved downstream MCP registry

#### Scenario: OpenCode runtime receives gateway MCP only
- **WHEN** the managed MCP gateway is enabled for an OpenCode chat turn
- **THEN** the managed OpenCode config contains only the gateway MCP connection entry
- **AND** OpenCode can reach all session-authorized downstream tools through that gateway

#### Scenario: iFlow runtime receives gateway MCP only
- **WHEN** the managed MCP gateway is enabled for an iFlow chat turn
- **THEN** the runtime syncs only the gateway MCP connection into the managed iFlow project config
- **AND** iFlow can use session-authorized downstream tools through that gateway
