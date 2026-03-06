# llm-settings (delta): auto-detect-openai-api-style

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

For OpenAI models, the daemon MUST internally preserve previously detected API style metadata only when the model identity and source connection fields remain unchanged. If the model name, source reference, source provider, source base_url, or source API key changes, the daemon MUST clear the previously detected API style metadata for affected models.

#### Scenario: Preserve internal API style metadata when model and source are unchanged

- **WHEN** client calls `PUT /api/v1/settings/llm` with an OpenAI model whose provider, source_id, model name, source provider, source base_url, and source API key are unchanged
- **THEN** the daemon keeps the previously detected internal API style metadata for that model

#### Scenario: Clear internal API style metadata when source connection changes

- **WHEN** client calls `PUT /api/v1/settings/llm` and changes an OpenAI source's base_url or API key
- **THEN** the daemon clears the previously detected internal API style metadata for OpenAI models using that source
