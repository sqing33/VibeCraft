# LLM Settings (delta): ui-llm-settings

## ADDED Requirements

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
- Source `provider` MUST be one of: `openai`, `anthropic`.
- If source `base_url` is non-empty, it MUST be a valid `http://` or `https://` URL.
- Model `id` MUST be non-empty and unique.
- Model `provider` MUST be one of: `openai`, `anthropic`.
- Model `model` MUST be non-empty.
- Model `source_id` MUST reference an existing source `id`.

If validation fails, the API MUST return HTTP 400 with an error message.

#### Scenario: Update LLM settings successfully

- **WHEN** client calls `PUT /api/v1/settings/llm` with a valid settings payload
- **THEN** the daemon persists the settings to `~/.config/vibecraft/config.json`
- **AND** the response returns the updated settings (with masked keys)

#### Scenario: Reject unknown source reference

- **WHEN** client calls `PUT /api/v1/settings/llm` and a model references a non-existent `source_id`
- **THEN** the daemon returns HTTP 400

### Requirement: LLM settings persistence MUST be safe by default

When persisting LLM settings to disk, the system MUST:
- write config with an atomic replace strategy (temp file + rename)
- set file permissions to `0600`

#### Scenario: Persisted config is private

- **WHEN** daemon writes `~/.config/vibecraft/config.json`
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

