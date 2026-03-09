## MODIFIED Requirements

### Requirement: Settings MUST expose a dedicated CLI tools tab
The UI MUST provide a dedicated `CLI 工具` tab for managing `Codex CLI`, `Claude Code`, `iFlow CLI`, and `OpenCode CLI`, including enablement, default model selection, and optional command path override.

#### Scenario: User manages four primary CLI tools
- **WHEN** user opens System Settings
- **THEN** the UI shows a `CLI 工具` tab
- **AND** the tab allows managing `Codex CLI`, `Claude Code`, `iFlow CLI`, and `OpenCode CLI`

### Requirement: Chat UI SHALL support runtime-first model selection
The chat page MUST let the user select a conversation runtime first and then choose a compatible model from that runtime's model pool.

The runtime list MUST include enabled CLI tools and available SDK providers in the same selector.

At minimum, when corresponding models exist, the selector MUST expose:

- `Codex CLI`
- `Claude Code`
- `iFlow CLI`
- `OpenCode CLI`
- `OpenAI SDK`
- `Anthropic SDK`

For CLI runtimes, the model selector MUST show only models whose provider is included in that tool's compatible protocol list.
For SDK runtimes, the model selector MUST only show models belonging to the selected provider.

#### Scenario: User chooses OpenCode then OpenAI model
- **WHEN** user selects `OpenCode CLI` in the chat composer
- **AND** the tool advertises OpenAI compatibility
- **THEN** the model selector includes OpenAI-compatible models

#### Scenario: User chooses OpenCode then Anthropic model
- **WHEN** user selects `OpenCode CLI` in the chat composer
- **AND** the tool advertises Anthropic compatibility
- **THEN** the model selector includes Anthropic-compatible models

#### Scenario: Active OpenCode session restores selector state
- **WHEN** an active session stores `cli_tool_id="opencode"`
- **THEN** the chat page restores the `OpenCode CLI` runtime option and current model selection

### Requirement: Chat composer MUST expose Codex reasoning effort control
The chat page MUST support selecting runtime/model per message and, for Codex CLI turns, sending the selected `reasoning_effort` to the daemon.

The `reasoning_effort` selector MUST remain a Codex-only control and MUST NOT be enabled for other CLI families, including OpenCode.

#### Scenario: OpenCode runtime disables reasoning effort selector
- **WHEN** the active composer runtime is `OpenCode CLI`
- **THEN** the reasoning effort selector remains visible but disabled
- **AND** the turn request does not include `reasoning_effort`
