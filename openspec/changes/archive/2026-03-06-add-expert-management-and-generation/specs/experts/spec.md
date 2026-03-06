## MODIFIED Requirements

### Requirement: Configuration Loading
The system MUST load expert configurations from `~/.config/vibe-tree/config.json` under the `experts` array. Each expert MUST have a unique `id`. The system MUST support provider types: `process` (local command), `openai` (OpenAI SDK), `anthropic` (Anthropic SDK), `demo` (testing).

In addition to runtime fields, an expert MAY include metadata fields used by the settings UI, including `description`, `category`, `avatar`, `managed_source`, `primary_model_id`, `secondary_model_id`, `enabled_skills`, and `fallback_on`.

The system MUST preserve builtin experts, MUST regenerate `llm-model` managed experts from current LLM settings, and MUST persist user-managed experts created from the expert settings tab.

#### Scenario: Load custom expert profile with model references
- **WHEN** config contains an expert with `managed_source="expert-profile"` and `primary_model_id="ui-designer"`
- **THEN** daemon startup hydrates the expert with provider/model/base_url/env derived from that LLM model
- **AND** the hydrated expert is available through the registry by its configured `id`

#### Scenario: Rebuild llm-model experts without losing custom experts
- **WHEN** the daemon saves updated LLM settings
- **THEN** it regenerates `llm-model` managed experts from the current model list
- **AND** keeps builtin and `expert-profile` experts intact

### Requirement: Expert List API
The system MUST provide `GET /api/v1/experts` returning all registered experts with safe fields only (no API keys). The response MUST include: id, label, provider, model for UI dropdown selection.

The system MUST also provide `GET /api/v1/settings/experts` returning expert settings metadata suitable for the settings UI, including description, category, avatar, managed source, primary/secondary model ids, fallback rules, enabled skills, readonly/editable markers, and generation metadata.

#### Scenario: Read full expert settings payload
- **WHEN** client requests `GET /api/v1/settings/experts`
- **THEN** the system returns all experts with safe metadata for display
- **AND** excludes API keys and raw env values from the payload

### Requirement: Expert registry supports fallback model execution
The system MUST allow an SDK expert to declare a secondary model and fallback conditions. When a primary SDK request fails, the runtime MUST retry once with the configured secondary model.

#### Scenario: Chat turn falls back to secondary model
- **WHEN** a chat turn uses an expert whose primary model request fails
- **AND** the expert has a configured secondary model
- **THEN** the system retries the turn with the secondary model
- **AND** stores the assistant message with the provider/model that actually succeeded

#### Scenario: Workflow execution falls back to secondary model
- **WHEN** a workflow node uses an SDK expert whose primary model request fails before completion
- **AND** the expert has a configured secondary model
- **THEN** the execution runner retries with the secondary model
- **AND** the log includes a fallback notice before the retry output

### Requirement: Expert settings can be saved independently from LLM settings
The system MUST provide `PUT /api/v1/settings/experts` so the UI can create, update, enable, disable, and delete user-managed experts without editing the models tab.

#### Scenario: Save custom experts from settings tab
- **WHEN** client submits a list of custom experts through `PUT /api/v1/settings/experts`
- **THEN** the daemon validates the payload, writes it to config, rebuilds runtime experts, and reloads the registry
- **AND** subsequent `GET /api/v1/experts` responses include the updated custom experts
