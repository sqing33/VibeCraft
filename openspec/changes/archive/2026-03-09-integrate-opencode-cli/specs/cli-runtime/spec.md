## MODIFIED Requirements

### Requirement: OpenAI-compatible CLI runtimes MUST inherit selected model source connection settings
When a CLI tool resolves a concrete `model_id`, the runtime MUST derive the selected model's source connection settings from persisted LLM settings and inject them into the CLI wrapper environment.

For OpenAI-compatible CLI tools, the runtime MUST provide the selected source's base URL and API key without requiring users to duplicate those values in a separate CLI-tool-specific config block.
For Anthropic-compatible CLI tools, the runtime MUST provide the selected source's base URL and API key without requiring users to duplicate those values in a separate CLI-tool-specific config block.

#### Scenario: OpenCode tool receives OpenAI source-backed connection settings
- **WHEN** a chat or repo analysis request selects the `opencode` CLI tool with an OpenAI-compatible `model_id`
- **THEN** the resolved CLI run spec includes the selected source's base URL and API key in environment variables or derived wrapper config usable by OpenCode
- **AND** the wrapper can authenticate without extra manual configuration

#### Scenario: OpenCode tool receives Anthropic source-backed connection settings
- **WHEN** a chat or repo analysis request selects the `opencode` CLI tool with an Anthropic-compatible `model_id`
- **THEN** the resolved CLI run spec includes the selected source's base URL and API key in environment variables or derived wrapper config usable by OpenCode
- **AND** the wrapper can authenticate without extra manual configuration

## ADDED Requirements

### Requirement: OpenCode wrapper MUST implement the standard CLI artifact contract
The `opencode` wrapper MUST write `summary.json` and `artifacts.json` for every completed run.

When the underlying CLI exposes a resumable session identifier, the wrapper MUST also write `session.json`.
When stdout or JSON events produce a final assistant response, the wrapper MUST persist it as `final_message.md`.

#### Scenario: OpenCode non-interactive run writes session and final message artifacts
- **WHEN** the `opencode` wrapper completes a non-interactive run
- **THEN** the wrapper writes `summary.json` and `artifacts.json`
- **AND** it writes `session.json` when a session id is present in OpenCode events
- **AND** it writes `final_message.md` from the final assistant output or result text
