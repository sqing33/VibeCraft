## MODIFIED Requirements

### Configuration Loading
The system MUST load expert configurations from `~/.config/vibe-tree/config.json` under the `experts` array. Each expert MUST have a unique `id`. The system MUST support provider types: `process` (local command), `cli` (CLI agent runtime), `openai` (helper SDK), `anthropic` (helper SDK), and `demo` (testing/helper only).

Experts intended for default chat, workflow, or orchestration execution MUST be representable as CLI-capable experts. SDK-backed experts MAY remain configured for helper-only operations and MUST be markable as `helper_only=true`.

In addition to runtime fields, an expert MAY include metadata fields used by the settings UI, including `description`, `category`, `avatar`, `managed_source`, `primary_model_id`, `secondary_model_id`, `enabled_skills`, `fallback_on`, `runtime_kind`, `cli_family`, and `helper_only`.

Expert configs MAY additionally store builder provenance fields, including `builder_session_id` and `builder_snapshot_id`, so a published expert can be traced back to its generation history.

#### Scenario: Load experts from config
- **WHEN** daemon starts and reads config.json
- **THEN** all experts in the `experts` array are registered
- **AND** each expert is accessible by its unique `id`

#### Scenario: Load CLI expert with runtime metadata
- **WHEN** config contains an expert with `provider="cli"`, `runtime_kind="cli"`, and `cli_family="codex"`
- **THEN** the daemon registers it as a CLI-capable expert
- **AND** callers can identify it as eligible for primary chat, workflow, and orchestration execution

#### Scenario: Load custom expert profile with model references
- **WHEN** config contains an expert with `managed_source="expert-profile"` and `primary_model_id="ui-designer"`
- **THEN** daemon startup hydrates the expert with provider/model/base_url/env derived from that helper LLM model when applicable
- **AND** the hydrated expert is available through the registry by its configured `id`

#### Scenario: Rebuild helper experts without losing custom experts
- **WHEN** the daemon saves updated helper LLM settings
- **THEN** it regenerates any managed helper experts from the current model list
- **AND** keeps builtin CLI experts and user-managed experts intact

### Expert Resolution
The system MUST provide `expert.Resolve()` to generate a `RunSpec` from expert config and node/session information.

`RunSpec` MUST support CLI mode, process mode, or helper SDK mode.

- For CLI mode, `RunSpec` MUST include the CLI family or adapter target, launch command/args or equivalent adapter inputs, environment, current working directory, timeout, and runtime hints needed to produce the standard artifact contract.
- For process mode, `RunSpec` MUST include command and substituted args.
- For helper SDK mode, `RunSpec` MUST include provider/model/messages and MUST only be returned for helper-only operations.

The system MUST inject `timeout_ms` into every resolved run spec.

#### Scenario: Resolve CLI-mode expert
- **WHEN** an expert with `provider="cli"` is resolved for a chat turn or project task
- **THEN** `RunSpec` contains the CLI adapter inputs, working directory, environment, and timeout needed to launch the CLI runtime

#### Scenario: Resolve helper SDK expert
- **WHEN** a helper-only SDK expert is resolved for a translation or other approved utility call
- **THEN** `RunSpec` contains provider/model/messages for that helper SDK request
- **AND** the resolved spec is marked as helper-only

### Expert List API
The system MUST provide `GET /api/v1/experts` returning all registered experts with safe fields only (no API keys). The response MUST include `id`, `label`, `provider`, `runtime_kind`, `helper_only`, and `model` or `cli_family` when available so callers can filter experts for primary AI surfaces versus helper utilities.

The system MUST also provide `GET /api/v1/settings/experts` returning expert settings metadata suitable for the settings UI, including description, category, avatar, managed source, primary/secondary model ids, fallback rules, enabled skills, readonly/editable markers, generation metadata, and runtime metadata.

#### Scenario: List experts for runtime-aware UI consumers
- **WHEN** client requests `GET /api/v1/experts`
- **THEN** all registered experts are returned with runtime filtering metadata
- **AND** API keys and sensitive env values are excluded

#### Scenario: Read full expert settings payload
- **WHEN** client requests `GET /api/v1/settings/experts`
- **THEN** the system returns all experts with safe metadata for display
- **AND** excludes API keys and raw env values from the payload

### Requirement: Expert registry supports fallback model execution
The system MUST allow a helper SDK expert to declare a secondary model and fallback conditions. When a helper SDK request fails, the runtime MUST retry once with the configured secondary model.

Default chat, workflow, and orchestration execution MUST NOT depend on this SDK fallback policy because those surfaces run through CLI runtime by default.

#### Scenario: Helper SDK call falls back to secondary model
- **WHEN** a helper SDK operation uses an expert whose primary model request fails
- **AND** the expert has a configured secondary model
- **THEN** the system retries that helper call with the secondary model
- **AND** stores or reports the provider/model that actually succeeded
