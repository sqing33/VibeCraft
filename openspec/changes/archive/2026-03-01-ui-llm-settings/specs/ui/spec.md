# UI (delta): ui-llm-settings

## ADDED Requirements

### Requirement: Settings uses tab navigation and includes LLM configuration

The UI MUST present the existing System Settings as a tabbed view.
The UI MUST provide at least two tabs:

- `连接与诊断`: contains daemon URL switching and diagnostics (version/paths/experts).
- `模型`: contains LLM Sources / Model Profiles configuration.

#### Scenario: User switches settings tabs

- **WHEN** user opens System Settings
- **THEN** the UI shows multiple tabs including `连接与诊断` and `模型`
- **AND** switching tabs updates the visible settings content

### Requirement: UI can edit and save LLM settings

In the `模型` tab, the UI MUST provide two sections:

- **Sources**: manage API sources (base URL + API key) without binding a model.
- **Models**: manage model profiles (model name, selected source, SDK provider: `codex(openai)` or `claudecode(anthropic)`).

The UI MUST save changes by calling `PUT /api/v1/settings/llm`.
After saving succeeds, the UI MUST refresh experts by calling `GET /api/v1/experts` so that workflow/node dropdowns can use the latest models.

#### Scenario: User adds a source and model and saves

- **WHEN** user creates a new Source and a new Model Profile referencing it
- **AND** user clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/llm`
- **AND** the UI shows a success toast
- **AND** the UI refreshes the experts list via `GET /api/v1/experts`
