## MODIFIED Requirements

### Requirement: Settings uses tab navigation and includes LLM configuration
The UI MUST present the existing System Settings as a tabbed view.
The UI MUST provide at least six tabs:

- `基本设置`: contains thinking translation configuration.
- `连接与诊断`: contains daemon URL switching and diagnostics (version/paths).
- `API 来源`: contains reusable API source configuration.
- `模型设置`: contains runtime-scoped model bindings for SDK and CLI runtimes.
- `CLI 工具`: contains tool-level CLI configuration.
- `专家`: contains expert list, expert details, and AI creation workflow.

#### Scenario: User switches settings tabs
- **WHEN** user opens System Settings
- **THEN** the UI shows multiple tabs including `基本设置`, `连接与诊断`, `API 来源`, `模型设置`, `CLI 工具`, and `专家`
- **AND** switching tabs updates the visible settings content

### Requirement: Basic settings tab can configure thinking translation
The `基本设置` tab MUST provide a `思考过程翻译` settings section.

The section MUST contain exactly these configurable fields:
- `翻译模型`: a selectable SDK runtime model
- `需要翻译的 AI 模型`: a multi-select list populated from all configured runtime model bindings

If no SDK runtime model exists, the UI MUST disable the translation configuration fields and guide the user to configure a translation model first in the `模型设置` tab.

If runtime models do not exist yet, the UI MUST disable the target model selector.

Saving the form MUST call `PUT /api/v1/settings/basic` and show success or failure feedback.

#### Scenario: Save thinking translation settings
- **WHEN** user selects a translation model, selects one or more target AI models, and clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/basic`
- **AND** the UI shows a success toast on success

#### Scenario: Basic settings disabled before SDK model configuration
- **WHEN** the user opens `基本设置` before configuring any SDK runtime model
- **THEN** the UI disables the thinking translation fields
- **AND** the UI shows guidance to configure a model in the `模型设置` tab first

### Requirement: UI can edit and save LLM settings
In the `API 来源` tab, the UI MUST organize reusable source settings as independent source cards without nested model rows.

Each Source card MUST manage:
- Source metadata: `id`, `label`, `base_url`, and source-level provider.
- Source secret: `api_key`.
- For `iflow`, source-level `auth_mode` metadata.

The UI MUST save source changes by calling `PUT /api/v1/settings/api-sources`.

#### Scenario: User edits an API source and saves
- **WHEN** user creates or edits an API source
- **AND** user clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/api-sources`
- **AND** the UI shows a success toast

### Requirement: Model profiles can be tested from the settings UI
In the `模型设置` tab, each runtime model row MUST provide a `测试` action when the bound source/provider pair supports SDK test probing.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the runtime model row's effective provider/model/source binding.

The UI MUST show success or failure feedback to the user.

#### Scenario: User tests a runtime model row
- **WHEN** user clicks `测试` on a runtime model row with complete provider/base_url/api_key/model configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the request uses the row's bound provider/model/source values
- **AND** the UI displays the result to the user

### Requirement: Settings MUST expose a dedicated CLI tools tab
The UI MUST provide a dedicated `CLI 工具` tab for managing `Codex CLI`, `Claude Code`, `iFlow CLI`, and `OpenCode CLI`, including enablement, optional command path override, and tool-specific health or login actions.

The `CLI 工具` tab MUST NOT be the primary editor for per-runtime model lists or default model bindings.

For `iFlow CLI`, the tab MUST expose browser-login actions and current official browser-auth status.

#### Scenario: User manages four primary CLI tools
- **WHEN** user opens System Settings
- **THEN** the UI shows a `CLI 工具` tab
- **AND** the tab allows managing `Codex CLI`, `Claude Code`, `iFlow CLI`, and `OpenCode CLI`

#### Scenario: User starts iFlow browser login from settings
- **WHEN** the user clicks the iFlow browser-login action
- **THEN** the frontend starts a daemon-managed auth session
- **AND** shows the real OAuth URL parsed from the iFlow terminal output
- **AND** allows the user to submit the returned authorization code

### Requirement: Chat UI SHALL support runtime-first model selection
The chat page MUST let the user select a conversation runtime first and then choose a compatible model from that runtime's saved model bindings.

The runtime list MUST include enabled CLI tools and available SDK runtimes in the same selector.

At minimum, when corresponding models exist, the selector MUST expose:

- `Codex CLI`
- `Claude Code`
- `iFlow CLI`
- `OpenCode CLI`
- `OpenAI SDK`
- `Anthropic SDK`

For every runtime, the model selector MUST show only the model bindings saved under that runtime.

#### Scenario: User chooses iFlow then iFlow model
- **WHEN** user selects `iFlow CLI` in the chat composer
- **THEN** the model selector only shows the models configured under runtime `iflow`
- **AND** the default value comes from that runtime's `default_model_id`

#### Scenario: User chooses OpenCode then OpenAI model
- **WHEN** user selects `OpenCode CLI` in the chat composer
- **THEN** the model selector includes only the models configured under runtime `opencode`
- **AND** selecting an OpenAI-bound model preserves that bound source at submit time

#### Scenario: Active OpenCode session restores selector state
- **WHEN** an active session stores `cli_tool_id="opencode"`
- **THEN** the chat page restores the `OpenCode CLI` runtime option and current model selection
