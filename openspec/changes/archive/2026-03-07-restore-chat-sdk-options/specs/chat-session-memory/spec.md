## MODIFIED Requirements

### Requirement: Chat sessions SHALL be persistent and resumable
The system MUST provide persistent and resumable chat sessions stored in the local state database. A session MUST have a stable `session_id` and metadata including title, selected runtime identity, status, and timestamps. The selected runtime identity MAY be either:

- a CLI tool + model combination, or
- a chat-capable SDK expert/model combination.

When the active chat runtime uses CLI execution, the session MUST remain resumable even if no provider-specific anchor exists.

When the active chat runtime uses SDK execution, the session MUST remain resumable through persisted messages, summary, and provider/model metadata.

#### Scenario: Create and list an SDK-backed session
- **WHEN** client calls `POST /api/v1/chat/sessions` with an SDK-backed `expert_id`
- **THEN** the new session is created successfully
- **AND** `GET /api/v1/chat/sessions` returns the session with persisted `expert_id`, `provider`, and `model`

### Requirement: Chat selection MUST support CLI tool and model as first-class inputs
The chat create-session and turn APIs MUST support both of the following input modes:

- `cli_tool_id` + compatible `model_id` for CLI-backed chat
- `expert_id` referencing a chat-capable SDK expert, optionally together with `model_id` for client-side model bookkeeping

For SDK-backed chat, the system MUST allow helper-only SDK experts when the resolved runtime is an SDK provider chat runtime. The system MUST continue rejecting non chat-capable runtimes such as `process` experts.

#### Scenario: Create session with codex tool and openai model
- **WHEN** client creates a session with `cli_tool_id="codex"` and an OpenAI-compatible `model_id`
- **THEN** the session is created successfully and uses that tool/model by default

#### Scenario: Create session with SDK helper expert
- **WHEN** client creates a session with `expert_id` set to an OpenAI- or Anthropic-backed helper expert
- **THEN** the session is created successfully
- **AND** subsequent turns run through the SDK provider path instead of CLI runtime

#### Scenario: Send turn with SDK helper expert
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with `expert_id` set to a chat-capable SDK helper expert
- **THEN** the turn is accepted
- **AND** the assistant response is generated through the resolved SDK provider

