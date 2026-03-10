## ADDED Requirements

### Requirement: Daemon MUST expose unified runtime model settings
The system MUST provide `GET /api/v1/settings/runtime-models` and `PUT /api/v1/settings/runtime-models` for managing model lists per runtime.

The runtime list MUST include at least:
- `sdk-openai`
- `sdk-anthropic`
- `codex`
- `claude`
- `iflow`
- `opencode`

Each runtime item MUST include:
- `id`
- `label`
- `runtime_kind` (`sdk` or `cli`)
- `provider_families`
- optional `default_model_id`
- `models[]`

Each model binding MUST include:
- `id`
- `label`
- `provider`
- `model`
- `source_id`

#### Scenario: Read runtime model settings
- **WHEN** client calls `GET /api/v1/settings/runtime-models`
- **THEN** the daemon returns all configured runtimes with their model lists and default model ids

#### Scenario: Save runtime model settings
- **WHEN** client calls `PUT /api/v1/settings/runtime-models` with valid runtime models
- **THEN** the daemon persists the new runtime model settings
- **AND** chat/runtime selectors can immediately use the updated model pools

### Requirement: Runtime model bindings MUST validate source compatibility
The system MUST validate every runtime model binding before persistence.

Validation rules:
- `id` MUST be non-empty and unique after normalization
- `model` MUST be non-empty
- `source_id` MUST reference an existing API source
- model `provider` MUST be supported by both the referenced source and the target runtime's `provider_families`
- `default_model_id`, when present, MUST reference a model inside the same runtime

#### Scenario: Reject model bound to missing source
- **WHEN** client saves a runtime model whose `source_id` does not exist
- **THEN** the daemon returns HTTP 400

#### Scenario: Reject model bound to incompatible runtime
- **WHEN** client saves a `claude` runtime model whose bound source provider is `openai`
- **THEN** the daemon returns HTTP 400

#### Scenario: Reject invalid runtime default model
- **WHEN** client saves runtime `codex` with `default_model_id` pointing to a model from another runtime
- **THEN** the daemon returns HTTP 400

### Requirement: Runtime model settings MUST drive runtime-first selection
Chat sessions and other runtime-aware surfaces MUST resolve selectable models from the saved runtime model settings instead of inferring models by filtering a shared provider pool.

#### Scenario: Chat runtime uses runtime-specific model pool
- **WHEN** user selects runtime `opencode`
- **THEN** the selectable model list comes from runtime `opencode` model bindings only
- **AND** the system does not infer additional models by scanning unrelated providers
