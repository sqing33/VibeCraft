## MODIFIED Requirements

### Requirement: Chat sessions MUST store CLI tool/model/session metadata
Chat sessions MUST persist the selected CLI tool, selected model id, and the last known CLI session reference needed to continue a CLI-native conversation.

The schema and read paths MUST remain compatible with databases created before these columns existed.

#### Scenario: Session stores codex tool and session reference
- **WHEN** a codex-backed chat session completes one turn
- **THEN** the session record includes `cli_tool_id`, `model_id`, and the persisted CLI session reference

#### Scenario: Old database is migrated before chat list query
- **WHEN** the daemon opens a database created before `cli_tool_id` existed
- **THEN** migration adds the missing columns before `ListChatSessions` queries them
