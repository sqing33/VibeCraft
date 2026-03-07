## ADDED Requirements

### Requirement: System-created analysis sessions SHALL preserve normal follow-up chat semantics
The system MUST allow automated repository-analysis sessions to later continue as normal user chat sessions.

The strict structured-report instruction MUST apply only to the automated final-report turn, not to every later follow-up turn in the same session.

#### Scenario: User asks a follow-up question after automated analysis
- **WHEN** the automated analysis has completed and the user sends a normal follow-up message in the same session
- **THEN** the assistant may answer naturally without being forced into the original full report template
- **AND** that reply does not become the official Repo Library report unless sync is explicitly triggered
