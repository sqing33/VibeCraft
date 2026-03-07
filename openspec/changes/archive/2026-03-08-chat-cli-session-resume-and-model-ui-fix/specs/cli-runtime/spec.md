## ADDED Requirements

### Requirement: CLI wrappers MUST write session.json for chat turns
CLI wrappers used by chat turns MUST write a `session.json` artifact whenever the underlying CLI exposes a resumable session/thread identifier.

#### Scenario: Wrapper writes session.json
- **WHEN** a chat turn completes and the CLI exposes a session/thread id
- **THEN** the wrapper writes `session.json` containing the tool id and session id
