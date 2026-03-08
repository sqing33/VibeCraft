## MODIFIED Requirements

### Requirement: Chat UI SHALL support tool-first model selection
The chat page MUST let the user select a conversation runtime first and then choose a compatible model from that runtime's model pool.

The runtime list MUST include enabled CLI tools and available SDK providers in the same selector.

At minimum, when corresponding models exist, the selector MUST expose:

- `Codex CLI`
- `Claude Code`
- `OpenAI SDK`
- `Anthropic SDK`

For CLI runtimes, the model selector MUST only show models compatible with that tool's protocol family.

For SDK runtimes, the model selector MUST only show models belonging to the selected provider.

#### Scenario: User chooses codex then openai model
- **WHEN** user selects `Codex CLI` in the chat composer
- **THEN** the model selector only shows OpenAI-compatible models

#### Scenario: User chooses OpenAI SDK
- **WHEN** user selects `OpenAI SDK` in the chat composer
- **THEN** the model selector only shows OpenAI provider models
- **AND** sending the message uses the SDK chat path instead of CLI runtime

#### Scenario: Active SDK session restores selector state
- **WHEN** an active session has `provider="openai"` or `provider="anthropic"` and no `cli_tool_id`
- **THEN** the chat page restores the corresponding SDK runtime option and current model selection

