## MODIFIED Requirements

### Requirement: Daemon exposes LLM settings update API

The system MUST provide `PUT /api/v1/settings/llm` to update the full LLM settings payload (sources + models).

Validation rules:
- Source `id` MUST be non-empty and unique.
- Source `provider` MAY be empty; when provided it MUST be one of: `openai`, `anthropic`.
- If source `base_url` is non-empty, it MUST be a valid `http://` or `https://` URL.
- Model `id` MUST be non-empty and unique after lowercase normalization.
- Model `provider` MUST be one of: `openai`, `anthropic`.
- Model `model` MUST be non-empty.
- Model `source_id` MUST reference an existing source `id`.
- The system MUST lowercase-trim model `id` and model `model` before persistence and before rebuilding the in-memory expert registry.
- The system MAY preserve the original display casing in model `label`.

If validation fails, the API MUST return HTTP 400 with an error message.

#### Scenario: Update LLM settings successfully

- **WHEN** client calls `PUT /api/v1/settings/llm` with a valid settings payload
- **THEN** the daemon persists the settings to `~/.config/vibe-tree/config.json`
- **AND** the response returns the updated settings (with masked keys)

#### Scenario: Reject unknown source reference

- **WHEN** client calls `PUT /api/v1/settings/llm` and a model references a non-existent `source_id`
- **THEN** the daemon returns HTTP 400

#### Scenario: Normalize mixed-case model identifiers on update

- **WHEN** client calls `PUT /api/v1/settings/llm` with model `id="GPT-5-CODEX"` and `model="GPT-5-CODEX"`
- **THEN** the persisted settings use `id="gpt-5-codex"` and `model="gpt-5-codex"`
- **AND** the model `label` may still preserve `GPT-5-CODEX`
