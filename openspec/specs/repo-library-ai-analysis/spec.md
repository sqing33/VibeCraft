# repo-library-ai-analysis Specification

## Purpose
TBD - created by archiving change repo-analysis-ai-chat. Update Purpose after archive.
## Requirements
### Requirement: Repo analysis MUST use a real AI chat session as the primary analysis runtime
The system MUST execute the primary repository analysis through a real CLI-backed chat session instead of generating the report solely from scripted report rendering heuristics.

The system MUST automatically create a chat session for the analysis and run the required turns without manual user intervention.

#### Scenario: User starts a repository analysis
- **WHEN** a client submits a repository analysis request
- **THEN** the backend automatically creates a chat session for that analysis
- **AND** the backend automatically launches one or more AI turns in that session to complete the analysis
- **AND** the user does not need to manually send any chat message for the analysis to finish

### Requirement: Automated repository analysis chat MUST remain user-continuable
The generated analysis chat session MUST remain visible and active after the automated analysis completes so the user can continue the conversation and refine the analysis result.

#### Scenario: User continues an automated analysis session
- **WHEN** an automated repository analysis has completed
- **THEN** the associated chat session remains available in the Chat UI
- **AND** the user can continue sending follow-up messages in that same session

### Requirement: Automated repository analysis MUST produce a final markdown report compatible with Repo Library post-processing
The final automated AI turn MUST produce the final analysis report as markdown in the expected Repo Library report structure so that downstream card extraction and search indexing can continue to operate.

Only the automated final-report turn MUST enforce the strict report template. Later user-driven follow-up turns in the linked chat session MUST remain free-form unless the user explicitly chooses to sync the latest reply back into Repo Library.

#### Scenario: Analysis chat produces final report markdown
- **WHEN** the automated analysis reaches its final report step
- **THEN** the assistant output is persisted as the snapshot report markdown
- **AND** the downstream card extraction step consumes that report without manual translation or editing

#### Scenario: Follow-up chat reply does not overwrite report automatically
- **WHEN** the user continues the linked analysis chat after the automated report has completed
- **THEN** the new assistant reply does not automatically replace the stored report
- **AND** Repo Library state changes only when the explicit sync action is invoked

