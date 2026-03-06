# Expert 注册表

## Purpose

Expert 是可执行的专家配置，定义了 provider、model、command、环境变量、超时策略等。Expert 注册表负责加载配置、解析模板、路由到正确的 runner。
## Requirements

### Configuration Loading

The system MUST load expert configurations from `~/.config/vibe-tree/config.json` under the `experts` array. Each expert MUST have a unique `id`. The system MUST support provider types: `process` (local command), `openai` (OpenAI SDK), `anthropic` (Anthropic SDK), `demo` (testing).

In addition to runtime fields, an expert MAY include metadata fields used by the settings UI, including `description`, `category`, `avatar`, `managed_source`, `primary_model_id`, `secondary_model_id`, `enabled_skills`, and `fallback_on`.

Expert configs MAY additionally store builder provenance fields, including `builder_session_id` and `builder_snapshot_id`, so a published expert can be traced back to its generation history.

#### Scenario: Load experts from config

- **WHEN** daemon starts and reads config.json
- **THEN** all experts in the `experts` array are registered
- **AND** each expert is accessible by its unique `id`

#### Scenario: Load custom expert profile with model references

- **WHEN** config contains an expert with `managed_source="expert-profile"` and `primary_model_id="ui-designer"`
- **THEN** daemon startup hydrates the expert with provider/model/base_url/env derived from that LLM model
- **AND** the hydrated expert is available through the registry by its configured `id`

#### Scenario: Rebuild llm-model experts without losing custom experts

- **WHEN** the daemon saves updated LLM settings
- **THEN** it regenerates `llm-model` managed experts from the current model list
- **AND** keeps builtin and `expert-profile` experts intact

### Template Substitution

The system MUST support `{{prompt}}` placeholder substitution with the node's prompt content. The system MUST support `{{workspace}}` placeholder substitution with the workflow's workspace path. The system MUST support `${ENV_VAR}` syntax for system environment variable injection.

#### Scenario: Prompt template substitution

- **WHEN** expert config has args `["-lc", "{{prompt}}"]` and node prompt is "echo hello"
- **THEN** RunSpec args become `["-lc", "echo hello"]`

#### Scenario: Environment variable injection

- **WHEN** expert env config has `{"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}"}`
- **AND** system environment variable ANTHROPIC_API_KEY is "sk-xxx"
- **THEN** RunSpec environment includes ANTHROPIC_API_KEY="sk-xxx"

### Expert Resolution

The system MUST provide `expert.Resolve()` to generate a `RunSpec` from expert config and node information. RunSpec MUST include command/args (process mode) or provider/model/messages (SDK mode). The system MUST inject timeout_ms into RunSpec.

#### Scenario: Resolve process-mode expert

- **WHEN** expert with provider "process" and command "bash" is resolved
- **THEN** RunSpec contains command="bash" with substituted args and timeout

### Expert List API

The system MUST provide `GET /api/v1/experts` returning all registered experts with safe fields only (no API keys). The response MUST include: id, label, provider, model for UI dropdown selection.

The system MUST also provide `GET /api/v1/settings/experts` returning expert settings metadata suitable for the settings UI, including description, category, avatar, managed source, primary/secondary model ids, fallback rules, enabled skills, readonly/editable markers, and generation metadata.

#### Scenario: List experts for UI

- **WHEN** client requests GET /api/v1/experts
- **THEN** all registered experts are returned
- **AND** API keys and sensitive env values are excluded

#### Scenario: Read full expert settings payload

- **WHEN** client requests `GET /api/v1/settings/experts`
- **THEN** the system returns all experts with safe metadata for display
- **AND** excludes API keys and raw env values from the payload

### Structured Output Support

The system MUST support `output_schema` configuration (e.g., `dag_v1`) for master structured output. The system MUST use `JSONSchemaV1()` to generate the structured output JSON schema for SDK calls.

#### Scenario: Master with structured output

- **WHEN** master expert has `output_schema: "dag_v1"` configured
- **THEN** SDK call includes the JSON schema for structured output
- **AND** response is parsed directly as DAG JSON

### Expert Validation

The system MUST provide `KnownExpertIDs()` returning the set of registered expert IDs. This MUST be used during DAG validation to verify that node expert_id values exist.

#### Scenario: Validate expert_id during DAG check

- **WHEN** DAG validation checks a node's expert_id
- **THEN** it verifies the ID exists in the set returned by `KnownExpertIDs()`

### Requirement: Expert registry supports runtime reload

The system MUST support reloading the in-memory expert registry at runtime after configuration updates (e.g. after saving LLM settings), without requiring a daemon restart.

#### Scenario: Reload updates listExperts API

- **WHEN** the daemon accepts an LLM settings update that changes model profiles
- **THEN** subsequent `GET /api/v1/experts` responses reflect the updated expert set

### Requirement: Expert registry supports fallback model execution

The system MUST allow an SDK expert to declare a secondary model and fallback conditions. When a primary SDK request fails, the runtime MUST retry once with the configured secondary model.

#### Scenario: Chat turn falls back to secondary model

- **WHEN** a chat turn uses an expert whose primary model request fails
- **AND** the expert has a configured secondary model
- **THEN** the system retries the turn with the secondary model
- **AND** stores the assistant message with the provider/model that actually succeeded

#### Scenario: Workflow execution falls back to secondary model

- **WHEN** a workflow node uses an SDK expert whose primary model request fails before completion
- **AND** the expert has a configured secondary model
- **THEN** the execution runner retries with the secondary model
- **AND** the log includes a fallback notice before the retry output

### Requirement: Expert settings can be saved independently from LLM settings

The system MUST provide `PUT /api/v1/settings/experts` so the UI can create, update, enable, disable, and delete user-managed experts without editing the models tab.

#### Scenario: Save custom experts from settings tab

- **WHEN** client submits a list of custom experts through `PUT /api/v1/settings/experts`
- **THEN** the daemon validates the payload, writes it to config, rebuilds runtime experts, and reloads the registry
- **AND** subsequent `GET /api/v1/experts` responses include the updated custom experts

#### Scenario: Save published expert with builder provenance

- **WHEN** client publishes a builder snapshot into an expert
- **THEN** the saved expert config includes the source builder session id and snapshot id
- **AND** later reads of expert settings expose those references to the UI
