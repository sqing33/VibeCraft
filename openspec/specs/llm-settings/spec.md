# llm-settings Specification

## Purpose
TBD - created by archiving change ui-llm-settings. Update Purpose after archive.
## Requirements
### Requirement: Daemon exposes LLM settings read API

The system MUST provide `GET /api/v1/settings/llm` returning the current LLM settings, including `sources[]` and `models[]`.

Each `source` item MUST include: `id`, `label`, `provider`, `base_url`, `has_key`, `masked_key`.

Each `model` item MUST include: `id`, `label`, `provider`, `model`, `source_id`.

#### Scenario: Fetch LLM settings

- **WHEN** client calls `GET /api/v1/settings/llm`
- **THEN** the response contains `sources` and `models` arrays
- **AND** each source includes `has_key` and `masked_key`

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

#### Scenario: Preserve internal API style metadata when model and source are unchanged

- **WHEN** client calls `PUT /api/v1/settings/llm` with an OpenAI model whose provider, source_id, model name, source provider, source base_url, and source API key are unchanged
- **THEN** the daemon keeps the previously detected internal API style metadata for that model

#### Scenario: Clear internal API style metadata when source connection changes

- **WHEN** client calls `PUT /api/v1/settings/llm` and changes an OpenAI source's base_url or API key
- **THEN** the daemon clears the previously detected internal API style metadata for OpenAI models using that source

### Requirement: LLM settings persistence MUST be safe by default

When persisting LLM settings to disk, the system MUST:
- write config with an atomic replace strategy (temp file + rename)
- set file permissions to `0600`

#### Scenario: Persisted config is private

- **WHEN** daemon writes `~/.config/vibe-tree/config.json`
- **THEN** the file mode is `0600`

### Requirement: LLM settings API MUST NOT expose plaintext API keys

The daemon MUST NOT return plaintext API keys in any HTTP response body. The daemon MUST only return `has_key` + `masked_key` for each source.

#### Scenario: Response does not contain plaintext key

- **WHEN** client calls `GET /api/v1/settings/llm`
- **THEN** the response does not include any `api_key` plaintext field

### Requirement: Updated LLM settings take effect without daemon restart

After `PUT /api/v1/settings/llm` succeeds, the daemon MUST update the in-memory expert registry so that model profiles are selectable/executable immediately.

#### Scenario: New model appears in experts list

- **WHEN** client saves settings containing a new model profile id `my-model`
- **THEN** `GET /api/v1/experts` includes an item with `id="my-model"`

### Requirement: Base URL is normalized by provider when calling SDK

When calling provider SDK with a configured `base_url`, the system MUST normalize the URL:
- For `openai`, the effective base URL MUST end with `/v1` (append if missing).
- For `anthropic`, the effective base URL MUST NOT end with `/v1` (remove if present).

#### Scenario: Normalize OpenAI base_url

- **WHEN** user sets `base_url` to `https://proxy.example.com`
- **AND** the system calls OpenAI SDK
- **THEN** the effective base URL is `https://proxy.example.com/v1`

#### Scenario: Normalize Anthropic base_url

- **WHEN** user sets `base_url` to `https://proxy.example.com/v1`
- **AND** the system calls Anthropic SDK
- **THEN** the effective base URL is `https://proxy.example.com`

### Requirement: LLM settings SHALL behave as a model pool for CLI tools and helper SDKs
Saved model profiles MUST become immediately available to the compatible CLI tool model pool and helper SDK flows, without requiring them to appear as primary execution experts.

#### Scenario: Saved OpenAI model becomes available under codex tool
- **WHEN** client saves an OpenAI-compatible model profile
- **THEN** that model becomes selectable for the `codex` CLI tool

