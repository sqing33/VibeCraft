## ADDED Requirements

### Requirement: Repo Library MUST derive repository context summary from the formal report
The system MUST derive a repository context summary from the formal report so the detail view can show stable repository-level analysis context without rendering the entire report body inline.

The summary MUST support at least:
- generated time
- stack overview
- backend summary
- frontend summary
- other modules summary

#### Scenario: Formal report includes stack and module language section
- **WHEN** a validated formal report contains the first-section summary for technology stack and module language
- **THEN** the backend derives the structured repository context summary from that report
- **AND** returns the summary in repository detail data for the selected snapshot
