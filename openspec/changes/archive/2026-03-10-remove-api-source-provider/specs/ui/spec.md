## MODIFIED Requirements

### Requirement: UI can edit and save LLM settings
In the `API 来源` tab, the UI MUST organize reusable source settings as independent source cards without nested model rows.

Each Source card MUST manage:
- Source metadata: `id`, `label`, `base_url`
- Source secret: `api_key`
- Optional source-level `auth_mode` metadata for iFlow usage

The Source card MUST NOT expose a source-level provider/type selector.

The UI MUST save source changes by calling `PUT /api/v1/settings/api-sources`.

#### Scenario: User edits an API source and saves
- **WHEN** user creates or edits an API source
- **AND** user clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/api-sources`
- **AND** the request body omits any source-level provider field
- **AND** the UI shows a success toast

### Requirement: Model profiles can be tested from the settings UI
In the `模型设置` tab, each runtime model card MUST provide a `测试` action when the model binding's effective provider supports SDK test probing.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the card's effective provider resolved from the model binding and the card's model ID as the effective model name.

The UI MUST show success or failure feedback to the user.

#### Scenario: User tests a runtime model card
- **WHEN** user clicks `测试` on a runtime model card with complete API source and model configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the request uses the card's model-level provider and model ID
- **AND** the UI displays the result to the user
