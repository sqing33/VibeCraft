## ADDED Requirements

### Requirement: OpenAI model calls SHALL auto-detect supported API style per model

For `provider=openai`, the system MUST support routing model calls through either `responses` or `chat/completions`.

When the API style for a model is unknown, the system MUST probe `responses` first. If `responses` fails with an endpoint-not-supported error, the system MUST probe `chat/completions`.

The first successful style MUST be used for the current request and MAY be persisted for later reuse.

#### Scenario: Responses is supported

- **WHEN** an OpenAI model has no saved API style
- **AND** the endpoint supports `responses`
- **THEN** the system uses `responses`

#### Scenario: Responses is not supported but chat completions is supported

- **WHEN** an OpenAI model has no saved API style
- **AND** `responses` returns an endpoint-not-supported error
- **AND** `chat/completions` succeeds
- **THEN** the system uses `chat/completions`

### Requirement: OpenAI API style SHALL be persisted per model and invalidated on config changes

The system MUST persist detected OpenAI API style metadata per LLM model in internal configuration.

The system MUST clear the saved style when any of the following changes for that model:
- provider
- source_id
- model name
- source provider
- source base_url
- source API key

#### Scenario: Preserve style when model config is unchanged

- **WHEN** the user saves LLM settings without changing a model's provider, source_id, model name, or source connection fields
- **THEN** the saved API style for that model remains available

#### Scenario: Invalidate style when source base_url changes

- **WHEN** the user changes the source base_url for a source referenced by an OpenAI model
- **THEN** the saved API style for that model is cleared before persistence

### Requirement: Runtime SHALL re-probe when saved API style no longer matches the gateway

If a model already has a saved API style but the actual request fails with an endpoint-not-supported error for that saved style, the system MUST clear the saved style and re-probe the other supported style once.

#### Scenario: Re-probe from responses to chat completions

- **WHEN** a model is saved as `responses`
- **AND** the current gateway now rejects `responses` with an endpoint-not-supported error
- **THEN** the system clears the saved style
- **AND** probes `chat/completions`
- **AND** persists the new successful style if probing succeeds

### Requirement: Chat-completions fallback SHALL degrade unsupported Responses-only features explicitly

When an OpenAI model uses `chat/completions`, the system MAY continue supporting ordinary text chat and plain text SDK execution.

When the system depends on a Responses-only capability, it MUST degrade explicitly:
- `previous_response_id` anchor reuse MUST be disabled
- OpenAI reasoning summary streaming MAY be absent
- strict structured output requests MUST return a clear error instead of silently degrading

#### Scenario: Plain chat remains available on chat completions

- **WHEN** a model is routed to `chat/completions`
- **THEN** ordinary text chat still produces assistant output

#### Scenario: Structured output rejects chat completions fallback

- **WHEN** a model is routed to `chat/completions`
- **AND** the request requires strict structured output
- **THEN** the system returns a clear error indicating a Responses-compatible endpoint is required
