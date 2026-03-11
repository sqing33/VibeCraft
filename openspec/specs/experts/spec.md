# Expert 注册表

## Purpose

Expert 是可执行的专家配置，定义了 runtime 类型、provider、model、command、环境变量、超时策略等。Expert 注册表负责加载配置、解析模板、路由到正确的 runner。
## Requirements

### Configuration Loading

The system MUST load expert configurations from `~/.config/vibe-tree/config.json` under the `experts` array. Each expert MUST have a unique `id`. The system MUST support provider types: `process` (local command), `cli` (CLI agent runtime), `openai` (helper SDK), `anthropic` (helper SDK), and `demo` (testing).

Experts intended for default chat, workflow, or orchestration execution MUST be representable as CLI-capable experts. SDK-backed experts MAY remain configured for helper-only operations and MUST be markable as `helper_only=true`.

In addition to runtime fields, an expert MAY include metadata fields used by the settings UI, including `description`, `category`, `avatar`, `managed_source`, `primary_model_id`, `secondary_model_id`, `enabled_skills`, `fallback_on`, `runtime_kind`, `cli_family`, and `helper_only`.

#### Scenario: Load experts from config
- **WHEN** daemon starts and reads config.json
- **THEN** all experts in the `experts` array are registered
- **AND** each expert is accessible by its unique `id`

#### Scenario: Load CLI expert with runtime metadata
- **WHEN** config contains an expert with `provider="cli"`, `runtime_kind="cli"`, and `cli_family="codex"`
- **THEN** the daemon registers it as a CLI-capable expert
- **AND** callers can identify it as eligible for primary chat, workflow, and orchestration execution

### Template Substitution

The system MUST support `{{prompt}}` placeholder substitution with the node's prompt content. The system MUST support `{{workspace}}` placeholder substitution with the workflow's workspace path. The system MUST support `${ENV_VAR}` syntax for system environment variable injection.

#### Scenario: Prompt template substitution
- **WHEN** expert config has args `["-lc", "{{prompt}}"]` and node prompt is "echo hello"
- **THEN** RunSpec args become `["-lc", "echo hello"]`

#### Scenario: Environment variable injection
- **WHEN** expert env config has `{"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}"}`
- **AND** system environment variable `ANTHROPIC_API_KEY` is set
- **THEN** RunSpec environment includes that value

### Expert Resolution

The system MUST provide `expert.Resolve()` to generate a `RunSpec` from expert config and node information. `RunSpec` MUST support CLI mode, process mode, or helper SDK mode. The system MUST inject `timeout_ms` into `RunSpec`.

#### Scenario: Resolve CLI-mode expert
- **WHEN** an expert with `provider="cli"` is resolved for a chat turn or project task
- **THEN** `RunSpec` contains the CLI adapter inputs, working directory, environment, and timeout needed to launch the CLI runtime

#### Scenario: Resolve process-mode expert
- **WHEN** expert with provider `process` and command `bash` is resolved
- **THEN** `RunSpec` contains `command="bash"` with substituted args and timeout

#### Scenario: Resolve helper SDK expert
- **WHEN** a helper-only SDK expert is resolved for a translation or other approved utility call
- **THEN** `RunSpec` contains provider/model/messages for that helper SDK request

### Expert List API

The system MUST provide `GET /api/v1/experts` returning all registered experts with safe fields only (no API keys). The response MUST include: `id`, `label`, `provider`, `model`, `runtime_kind`, `cli_family`, `helper_only` for UI selection and filtering.

The system MUST also provide `GET /api/v1/settings/experts` returning expert settings metadata suitable for the settings UI, including description, category, avatar, managed source, primary/secondary model ids, fallback rules, enabled skills, readonly/editable markers, generation metadata, and runtime metadata.

#### Scenario: List experts for runtime-aware UI
- **WHEN** client requests `GET /api/v1/experts`
- **THEN** all registered experts are returned with runtime filtering metadata
- **AND** API keys and sensitive env values are excluded

### Structured Output Support

The system MUST support `output_schema` configuration (e.g., `dag_v1`) for master structured output. CLI experts MAY satisfy this requirement through their wrapper/artifact contract, and helper SDK experts MAY continue using SDK-native structured output where supported.

#### Scenario: Master with structured output
- **WHEN** master expert has `output_schema: "dag_v1"` configured
- **THEN** the runtime produces machine-readable planning output for DAG parsing

### Expert Validation

The system MUST provide `KnownExpertIDs()` returning the set of registered expert IDs. This MUST be used during DAG validation to verify that node `expert_id` values exist.

#### Scenario: Validate expert_id during DAG check
- **WHEN** DAG validation checks a node's expert_id
- **THEN** it verifies the ID exists in the set returned by `KnownExpertIDs()`

### Requirement: Expert registry supports runtime reload

The system MUST support reloading the in-memory expert registry at runtime after configuration updates without requiring a daemon restart.

#### Scenario: Reload updates listExperts API
- **WHEN** the daemon accepts a settings update that changes model profiles or custom experts
- **THEN** subsequent `GET /api/v1/experts` responses reflect the updated expert set

### Requirement: Expert registry supports fallback model execution

The system MUST allow a helper SDK expert to declare a secondary model and fallback conditions. When a helper SDK request fails, the runtime MUST retry once with the configured secondary model. Primary chat, workflow, and orchestration execution MUST NOT depend on this SDK fallback policy because those surfaces run through CLI runtime by default.

#### Scenario: Helper SDK call falls back to secondary model
- **WHEN** a helper SDK request uses an expert whose primary model request fails
- **AND** the expert has a configured secondary model
- **THEN** the system retries that helper request with the secondary model

### Requirement: Expert list MUST not treat model mirrors as the primary execution surface
The system MAY keep `llm-model` expert mirrors for helper or builder flows, but primary execution selection MUST be tool-first and helper-only entries MUST be distinguishable in the public payload.

#### Scenario: Public expert payload marks helper-only entries
- **WHEN** client requests `GET /api/v1/experts`
- **THEN** helper-only mirrored model entries are identifiable
- **AND** UI clients can exclude them from primary chat execution selection

### Requirement: Expert enabled_skills MUST constrain runtime skill injection
When an expert declares `enabled_skills`, the system MUST treat that list as a runtime constraint for CLI chat sessions that support skill guidance.

If an expert does not declare `enabled_skills`, the runtime MAY use the full discovered skill set.
If an expert declares `enabled_skills`, the runtime MUST use only the intersection of discovered skills and the expert list.

#### Scenario: Expert without enabled_skills uses discovered defaults
- **WHEN** a chat session runs with an expert that has no `enabled_skills`
- **THEN** the runtime uses the full discovered skill set

#### Scenario: Expert enabled_skills narrows runtime set
- **WHEN** a chat session runs with an expert whose `enabled_skills` contains `ui-ux-pro-max`
- **AND** the discovered skill catalog contains `ui-ux-pro-max` and `worktree-lite`
- **THEN** the runtime injects only `ui-ux-pro-max`

### Requirement: Chat turns MUST enforce expert timeout_ms
When a chat turn is executed using an expert that resolves to a non-zero timeout, the daemon MUST enforce that timeout for the turn execution.

The daemon MUST ignore HTTP request cancellation for long-running turns, but it MUST still stop the turn when the expert timeout elapses.

#### Scenario: Chat turn stops after expert timeout
- **WHEN** the active chat expert resolves with `timeout_ms > 0`
- **AND** the turn execution exceeds that timeout
- **THEN** the daemon stops the turn execution
- **AND** the persisted turn timeline reaches a terminal state (not running)

