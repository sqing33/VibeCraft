## ADDED Requirements

### Requirement: Daemon MUST expose dedicated API source settings
The system MUST provide `GET /api/v1/settings/api-sources` and `PUT /api/v1/settings/api-sources` for managing reusable API source definitions independently from runtime model lists.

Each source item MUST include:
- `id`
- `label`
- `provider`
- optional `base_url`
- masked-key metadata (`has_key`, `masked_key`) on read

For `iflow` sources, each item MUST additionally support:
- `auth_mode` (`browser` or `api_key`)
- browser-auth status metadata derived from the managed iFlow home

#### Scenario: Read API sources
- **WHEN** client calls `GET /api/v1/settings/api-sources`
- **THEN** the daemon returns all saved API sources without plaintext API keys
- **AND** each source includes provider, base URL, and masked-key metadata

#### Scenario: Update API sources
- **WHEN** client calls `PUT /api/v1/settings/api-sources` with a valid source list
- **THEN** the daemon persists the new source list
- **AND** later runtime model resolution can reference those source ids

### Requirement: API source settings MUST validate provider-specific fields
The system MUST validate API sources before persistence.

Validation rules:
- `provider` MUST be one of `openai`, `anthropic`, or `iflow`
- `id` MUST be non-empty and unique
- `base_url`, when provided, MUST be a valid `http://` or `https://` URL
- `iflow` sources MUST persist `auth_mode`, defaulting to `browser`

#### Scenario: Reject duplicate source id
- **WHEN** client saves two API sources with the same `id`
- **THEN** the daemon returns HTTP 400

#### Scenario: Reject unsupported provider
- **WHEN** client saves an API source with provider other than `openai`, `anthropic`, or `iflow`
- **THEN** the daemon returns HTTP 400

### Requirement: API source responses MUST NOT expose plaintext API keys
The daemon MUST NOT include plaintext API keys in `GET /api/v1/settings/api-sources` responses.

#### Scenario: API key remains masked
- **WHEN** client reads API source settings after a source has a saved key
- **THEN** the response includes `has_key=true` and a masked key string
- **AND** the plaintext key is absent from the response body
