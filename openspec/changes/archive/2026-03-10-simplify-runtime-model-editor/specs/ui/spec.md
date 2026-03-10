## ADDED Requirements

### Requirement: Runtime model editor MUST use simplified model cards
In the `模型设置` tab, each runtime MUST render its models as responsive cards in a multi-column grid with up to three columns.

Each model card MUST:
- show `模型` as the card title
- provide `设为默认`, `测试`, `删除` actions in the card header
- expose exactly three editable rows: `模型`, `显示名称`, `API 来源`

The `模型设置` tab MUST NOT expose:
- a runtime-level `默认模型` dropdown
- a protocol-family editor
- a separate actual-model editor

#### Scenario: User sets a runtime default from a model card
- **WHEN** the user clicks `设为默认` on a model card
- **THEN** the UI updates that runtime's `default_model_id` to the card's model id
- **AND** the runtime-level default selector is not shown elsewhere in the tab

#### Scenario: User edits a simplified model card
- **WHEN** the user edits only `模型`, `显示名称`, and `API 来源` on a model card and saves
- **THEN** the UI calls `PUT /api/v1/settings/runtime-models`
- **AND** the save succeeds without requiring separate protocol-family or actual-model inputs

## MODIFIED Requirements

### Requirement: Model profiles can be tested from the settings UI
In the `模型设置` tab, each runtime model card MUST provide a `测试` action when the selected API source supports SDK test probing.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the card's effective provider resolved from the selected API source and the card's model ID as the effective model name.

The UI MUST show success or failure feedback to the user.

#### Scenario: User tests a runtime model card
- **WHEN** user clicks `测试` on a runtime model card with complete API source and model configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the request uses the card's selected source provider and model ID
- **AND** the UI displays the result to the user
