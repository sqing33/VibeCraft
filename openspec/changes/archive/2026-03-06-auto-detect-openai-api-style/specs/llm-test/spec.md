# llm-test (delta): auto-detect-openai-api-style

## MODIFIED Requirements

### Requirement: Daemon exposes LLM test API

The system MUST provide `POST /api/v1/settings/llm/test` to perform a short SDK-based test call.

The request body MUST include:
- `provider`: `openai` or `anthropic`
- `model`: non-empty model name
- `base_url`: optional base URL (http/https)
- `api_key`: optional API key (preferred for unsaved draft)
- `source_id`: optional source id (used to lookup saved key/base_url)

The request MUST provide at least one of: `api_key`, `source_id`.

Before calling the provider SDK, the system MUST lowercase-trim the effective `model` value.

For `provider=openai`, the daemon MUST probe the supported API style automatically when needed. If the test request matches a saved model profile, the daemon SHOULD persist the detected API style for that model profile. If the test request is only an unsaved draft that cannot be mapped to a saved model profile, the daemon MAY probe API style for the test call but MUST NOT persist it.

The response on success MUST include:
- `ok: true`
- `output`: a short output string (truncated)
- `latency_ms`: request latency in milliseconds

The API MUST return HTTP 400 for invalid request payloads and HTTP 500 for provider/network errors.

#### Scenario: Test OpenAI model persists detected style for saved profile

- **WHEN** client posts a valid `openai` test payload that matches a saved model profile
- **AND** the daemon successfully detects a supported API style
- **THEN** the response is HTTP 200 with `ok: true`
- **AND** the daemon persists the detected API style for that model profile

#### Scenario: Unsaved draft test does not persist detected style

- **WHEN** client posts a valid `openai` test payload that does not match a saved model profile
- **AND** the daemon successfully detects a supported API style
- **THEN** the response is HTTP 200 with `ok: true`
- **AND** the daemon does not persist the detected API style
