## ADDED Requirements

### Requirement: Chat sessions MUST store CLI tool/model/session metadata
Chat sessions MUST persist enough metadata to continue a CLI-native conversation, including the selected CLI tool, selected model id, and the last known CLI session reference.

#### Scenario: Session stores codex tool and session reference
- **WHEN** a user creates a codex-backed chat session and completes one turn
- **THEN** the session record includes the selected `cli_tool_id`, `model_id`, and the persisted Codex session/thread reference
