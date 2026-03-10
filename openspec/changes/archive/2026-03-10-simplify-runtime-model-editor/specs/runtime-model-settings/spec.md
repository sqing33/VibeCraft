## ADDED Requirements

### Requirement: Runtime model settings MUST accept simplified model editor payloads
The system MUST allow runtime model save requests to omit advanced binding fields that can be derived from the selected API source and model identifier.

When a runtime model save payload omits advanced fields:
- `label` MUST default to `id` when empty
- `model` MUST default to the normalized `id`
- `provider` MUST default to the referenced `source_id` provider
- source compatibility and runtime compatibility validation MUST still be enforced before persistence

#### Scenario: Save simplified runtime model binding
- **WHEN** client saves a runtime model with `id="gpt-5-codex"`, empty `label`, empty `provider`, empty `model`, and `source_id="openai-default"`
- **THEN** the daemon persists the binding with `label="gpt-5-codex"`
- **AND** the daemon persists `model="gpt-5-codex"`
- **AND** the daemon persists `provider="openai"`

#### Scenario: Reject simplified model bound to incompatible source
- **WHEN** client saves runtime `claude` with a model bound to source `openai-default`
- **THEN** the daemon returns HTTP 400
