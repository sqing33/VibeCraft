## MODIFIED Requirements

### Requirement: Store MUST persist Repo Library entities
The store SHALL persist repository sources, repository snapshots, analysis runs, knowledge cards, card evidence, and search query history for Repo Library.

Repo analysis runs SHALL also persist the associated chat session identifier and the selected runtime/tool/model metadata when the analysis is AI-chat driven.

#### Scenario: Store creates Repo Library records
- **WHEN** the backend creates a repository source, snapshot, analysis run, and associated automated chat session
- **THEN** the store persists those records with stable identifiers
- **AND** later queries can retrieve the analysis run together with its linked chat session metadata

#### Scenario: Store lists repository summaries
- **WHEN** the UI requests Repo Library repositories
- **THEN** the store returns repository-level summaries with latest snapshot and latest analysis metadata
- **AND** the records are ordered by recent activity
