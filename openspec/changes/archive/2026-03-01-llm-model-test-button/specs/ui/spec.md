# UI (delta): llm-model-test-button

## ADDED Requirements

### Requirement: Model profiles can be tested from the settings UI

In the `模型` settings tab, each model profile card MUST provide a `测试` button located to the left of the delete button.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the model card's current draft provider/model/base_url/api_key values.

The UI MUST show success or failure feedback to the user (e.g. toast).

#### Scenario: User tests a model profile

- **WHEN** user clicks `测试` on a model card with complete configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the UI displays the result to the user
