## ADDED Requirements

### Requirement: Chat selection MUST support CLI tool and model as first-class inputs
The chat create-session and turn APIs MUST support `cli_tool_id` and `model_id` so the client can choose a CLI tool first and then select a model within that tool.

The system MUST validate that the selected model belongs to the selected tool's bound protocol family.

#### Scenario: Create session with codex tool and openai model
- **WHEN** client creates a chat session with `cli_tool_id="codex"` and an OpenAI-compatible `model_id`
- **THEN** the session is created successfully
- **AND** subsequent turns default to that tool/model combination unless overridden

#### Scenario: Reject incompatible tool and model
- **WHEN** client posts a turn with `cli_tool_id="claude"` and an OpenAI-compatible model
- **THEN** the daemon rejects the request with a validation error
