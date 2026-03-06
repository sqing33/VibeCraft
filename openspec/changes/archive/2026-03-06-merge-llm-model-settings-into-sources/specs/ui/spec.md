## MODIFIED Requirements

### Requirement: UI can edit and save LLM settings

In the `模型` tab, the UI MUST organize LLM settings by API Source.

Each Source card MUST manage:

- Source metadata: `id`, `label`, `base_url`, and source-level SDK provider.
- Source secret: `api_key`.
- A nested model list rendered directly under the API Key field.

The UI MUST NOT present a separate top-level `Models` section.

Each nested model row MUST allow the user to enter one model ID string. The UI MUST use the entered text as the display label and MUST derive the persisted model `id` and `model` by lowercasing the trimmed input.

The UI MUST save changes by calling `PUT /api/v1/settings/llm`.
After saving succeeds, the UI MUST refresh experts by calling `GET /api/v1/experts` so that workflow/node/chat dropdowns can use the latest models.

#### Scenario: User adds a source model inside the source card and saves

- **WHEN** user creates or edits an API Source
- **AND** user adds a model row under that Source card
- **AND** user clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/llm`
- **AND** the UI shows a success toast
- **AND** the UI refreshes the experts list via `GET /api/v1/experts`

#### Scenario: Mixed-case model input is normalized on save

- **WHEN** user enters a mixed-case model ID such as `GPT-5-CODEX` in a Source card
- **AND** user clicks Save
- **THEN** the UI submits lowercase `id` and `model` values such as `gpt-5-codex`
- **AND** the UI keeps the original input as the display label

### Requirement: Model profiles can be tested from the settings UI

In the `模型` settings tab, each nested Source model row MUST provide a `测试` button located to the left of the delete button.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the Source row's current provider/base_url/api_key values and the model row's lowercase-normalized model ID.

The UI MUST show success or failure feedback to the user (e.g. toast).

#### Scenario: User tests a source model row

- **WHEN** user clicks `测试` on a model row with complete Source SDK/API Key/model configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the request uses the Source card's SDK and the model row's lowercase-normalized model ID
- **AND** the UI displays the result to the user

### Requirement: LLM model profiles require a valid Source

Because model rows are edited inside a Source card, each model profile MUST be implicitly bound to exactly one Source.

The UI MUST prevent saving or testing LLM settings when:

- the Source card is missing a valid SDK provider, or
- any nested model row is missing a non-empty model ID.

#### Scenario: Saving is blocked when a nested model ID is missing

- **WHEN** the user clicks Save while any Source card contains an empty model row
- **THEN** the UI shows an error toast describing the missing model ID
- **AND** does not submit the settings update request

#### Scenario: Testing is blocked when Source SDK is missing

- **WHEN** the user clicks `测试` for a model row whose Source card has no SDK provider
- **THEN** the UI shows an error toast describing the missing Source SDK
- **AND** does not submit the test request
