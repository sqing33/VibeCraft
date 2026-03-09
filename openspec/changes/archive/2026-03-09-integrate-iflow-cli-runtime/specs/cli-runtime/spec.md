## ADDED Requirements

### Requirement: OpenAI-compatible CLI runtimes MUST inherit selected model source connection settings
When a CLI tool resolves a concrete `model_id`, the runtime MUST derive the selected model's source connection settings from persisted LLM settings and inject them into the CLI wrapper environment.

For OpenAI-compatible CLI tools, the runtime MUST provide the selected source's base URL and API key without requiring users to duplicate those values in a separate CLI-tool-specific config block.

#### Scenario: IFLOW tool receives source-backed OpenAI connection settings
- **WHEN** a chat or repo analysis request selects the `iflow` CLI tool with an OpenAI-compatible `model_id`
- **THEN** the resolved CLI run spec includes the selected source's base URL and API key in environment variables usable by the wrapper
- **AND** the wrapper can authenticate against the selected source without extra manual configuration

### Requirement: IFLOW wrapper MUST implement the standard CLI artifact contract
The `iflow` wrapper MUST write `summary.json` and `artifacts.json` for every completed run.

When the underlying CLI exposes a resumable session identifier, the wrapper MUST also write `session.json`.
When stdout produces a final assistant response, the wrapper MUST persist it as `final_message.md`.

#### Scenario: IFLOW non-interactive run writes session and final message artifacts
- **WHEN** the `iflow` wrapper completes a non-interactive run with `--output-file`
- **THEN** the wrapper writes `summary.json` and `artifacts.json`
- **AND** it writes `session.json` from the returned execution info when `session-id` is present
- **AND** it writes `final_message.md` from the assistant output
