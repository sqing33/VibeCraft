## MODIFIED Requirements

### Requirement: Settings MUST expose a dedicated CLI tools tab
The UI MUST provide a dedicated `CLI 工具` tab for managing `Codex CLI`, `Claude Code`, and `iFlow CLI`, including enablement, default model selection, and optional command path override.

#### Scenario: User manages codex, claude, and iflow tools
- **WHEN** user opens System Settings
- **THEN** the UI shows a `CLI 工具` tab
- **AND** the tab allows managing all three primary execution tools

### Requirement: Chat UI SHALL support runtime-first model selection
The chat page MUST let the user select a conversation runtime first and then choose a compatible model from that runtime's model pool.

The runtime list MUST include enabled CLI tools and available SDK providers in the same selector.

At minimum, when corresponding models exist, the selector MUST expose:

- `Codex CLI`
- `Claude Code`
- `iFlow CLI`
- `OpenAI SDK`
- `Anthropic SDK`

For CLI runtimes, the model selector MUST only show models compatible with that tool's protocol family.

For SDK runtimes, the model selector MUST only show models belonging to the selected provider.

#### Scenario: User chooses iflow then openai model
- **WHEN** user selects `iFlow CLI` in the chat composer
- **THEN** the model selector only shows OpenAI-compatible models

#### Scenario: User chooses OpenAI SDK
- **WHEN** user selects `OpenAI SDK` in the chat composer
- **THEN** the model selector only shows OpenAI provider models
- **AND** sending the message uses the SDK chat path instead of CLI runtime

#### Scenario: Active IFLOW session restores selector state
- **WHEN** an active session stores `cli_tool_id="iflow"`
- **THEN** the chat page restores the `iFlow CLI` runtime option and current model selection
