## MODIFIED Requirements

### Requirement: iFlow runtime MUST use official authentication only
The `iflow` runtime MUST NOT depend on generic OpenAI-compatible source/model semantics.

The runtime MUST resolve credentials from an `iflow` API source binding selected by the active runtime model.

Supported iFlow source auth inputs are:
- browser-auth state persisted in the managed iFlow home
- explicit official iFlow API key from the selected `iflow` API source

The runtime MUST NOT reuse OpenAI or Anthropic source credentials for iFlow.

#### Scenario: iFlow turn uses official browser auth
- **WHEN** a chat turn selects the `iflow` CLI tool with a model bound to an `iflow` source whose `auth_mode` is `browser`
- **THEN** the runtime injects the managed iFlow home into the wrapper environment
- **AND** the wrapper reuses the official browser-auth state without consulting OpenAI or Anthropic source credentials

#### Scenario: iFlow turn uses official API key
- **WHEN** a chat turn selects the `iflow` CLI tool with a model bound to an `iflow` source whose `auth_mode` is `api_key`
- **THEN** the runtime injects the official iFlow API key and official base URL from that selected source
- **AND** the wrapper does not require a matching shared OpenAI/Anthropic source entry

#### Scenario: iFlow turn receives effective skills
- **WHEN** an iFlow chat turn runs with enabled skill bindings
- **THEN** the runtime appends effective skill instructions to the system prompt before launching the CLI

For OpenAI-compatible CLI tools, the runtime MUST provide the selected bound source's base URL and API key without requiring users to duplicate those values in a separate CLI-tool-specific config block.
For Anthropic-compatible CLI tools, the runtime MUST provide the selected bound source's base URL and API key without requiring users to duplicate those values in a separate CLI-tool-specific config block.

#### Scenario: OpenCode tool receives OpenAI source-backed connection settings
- **WHEN** a chat or repo analysis request selects the `opencode` CLI tool with an OpenAI-compatible runtime model binding
- **THEN** the resolved CLI run spec includes the selected source's base URL and API key in environment variables or derived wrapper config usable by OpenCode
- **AND** the wrapper can authenticate without extra manual configuration

#### Scenario: OpenCode tool receives Anthropic source-backed connection settings
- **WHEN** a chat or repo analysis request selects the `opencode` CLI tool with an Anthropic-compatible runtime model binding
- **THEN** the resolved CLI run spec includes the selected source's base URL and API key in environment variables or derived wrapper config usable by OpenCode
- **AND** the wrapper can authenticate without extra manual configuration

## ADDED Requirements

### Requirement: CLI runtimes MUST use application-managed config roots
CLI runtimes MUST apply source-specific configuration through application-managed config roots or per-run config files stored outside the project workspace and outside the user's default global CLI config paths.

Managed config state MUST live under the application's data directory.
The runtime MUST NOT directly overwrite project-local `.codex`, `.claude`, `.iflow`, or user-global default config files as part of normal execution.

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
