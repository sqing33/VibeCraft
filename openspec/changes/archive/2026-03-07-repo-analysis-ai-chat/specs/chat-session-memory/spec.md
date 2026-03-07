## ADDED Requirements

### Requirement: System-created analysis sessions SHALL behave like normal resumable chat sessions
The system MUST allow background product flows to create chat sessions that are later visible and resumable in the normal Chat UI.

#### Scenario: Repo analysis creates a system chat session
- **WHEN** Repo Library starts an automated AI analysis
- **THEN** the system creates a persistent chat session without manual user input
- **AND** that session appears in normal chat session listings
- **AND** the user can continue the session manually after automation completes
