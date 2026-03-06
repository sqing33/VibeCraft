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

The response on success MUST include:
- `ok: true`
- `output`: a short output string (truncated)
- `latency_ms`: request latency in milliseconds

The API MUST return HTTP 400 for invalid request payloads and HTTP 500 for provider/network errors.

#### Scenario: Test OpenAI model succeeds

- **WHEN** client posts a valid `openai` test payload
- **THEN** the response is HTTP 200 with `ok: true`
- **AND** the response includes `output` and `latency_ms`

#### Scenario: Reject empty api_key

- **WHEN** client posts a payload with neither `api_key` nor `source_id`
- **THEN** the daemon returns HTTP 400

#### Scenario: Normalize mixed-case model name before test call

- **WHEN** client posts `model="GPT-5-CODEX"`
- **THEN** the daemon lowercases it before calling the provider SDK
