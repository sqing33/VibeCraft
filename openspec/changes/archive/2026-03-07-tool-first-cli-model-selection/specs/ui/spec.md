## MODIFIED Requirements

### Requirement: Settings uses tab navigation and includes LLM configuration
The UI MUST present the existing System Settings as a tabbed view.
The UI MUST provide at least five tabs:
- `基本设置`
- `连接与诊断`
- `CLI 工具`
- `模型`
- `专家`

#### Scenario: User sees CLI tools tab
- **WHEN** user opens System Settings
- **THEN** the UI shows a dedicated `CLI 工具` tab for `Codex CLI` and `Claude Code`

### Requirement: Chat UI SHALL support tool-first model selection
The chat page MUST allow the user to select a CLI tool first and then select a model from that tool's compatible model pool.

#### Scenario: User chooses codex then openai model
- **WHEN** user selects `Codex CLI` in the chat composer
- **THEN** the model selector only shows OpenAI-compatible models

#### Scenario: User chooses claude then anthropic model
- **WHEN** user selects `Claude Code` in the chat composer
- **THEN** the model selector only shows Anthropic-compatible models
