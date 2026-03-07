## ADDED Requirements

### Requirement: Store MUST persist Repo Library entities
The store SHALL persist repository sources, repository snapshots, analysis runs, knowledge cards, card evidence, and search query history for Repo Library.

#### Scenario: Store creates Repo Library records
- **WHEN** the backend creates a repository source, snapshot, and analysis run
- **THEN** the store persists those records with stable identifiers
- **AND** later queries can retrieve them by repository, snapshot, or run id

#### Scenario: Store lists repository summaries
- **WHEN** the UI requests Repo Library repositories
- **THEN** the store returns repository-level summaries with latest snapshot and latest analysis metadata
- **AND** the records are ordered by recent activity
