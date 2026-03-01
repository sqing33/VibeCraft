# Expert 注册表

## Purpose

Expert 是可执行的专家配置，定义了 provider、model、command、环境变量、超时策略等。Expert 注册表负责加载配置、解析模板、路由到正确的 runner。

## Requirements

### Configuration Loading

The system MUST load expert configurations from `~/.config/vibe-tree/config.json` under the `experts` array. Each expert MUST have a unique `id`. The system MUST support provider types: `process` (local command), `openai` (OpenAI SDK), `anthropic` (Anthropic SDK), `demo` (testing).

#### Scenario: Load experts from config

- **WHEN** daemon starts and reads config.json
- **THEN** all experts in the `experts` array are registered
- **AND** each expert is accessible by its unique `id`

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

#### Scenario: List experts for UI

- **WHEN** client requests GET /api/v1/experts
- **THEN** all registered experts are returned
- **AND** API keys and sensitive env values are excluded

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
