# repo-library-ui Specification

## Purpose
TBD - created by archiving change add-repo-library. Update Purpose after archive.
## Requirements
### Requirement: UI MUST provide a top-level Repo Library navigation entry
The UI MUST expose Repo Library as a top-level navigation destination alongside existing primary product lanes.

#### Scenario: User navigates to Repo Library
- **WHEN** the application loads and the user chooses the Repo Library entry
- **THEN** the frontend navigates to Repo Library pages without requiring manual URL editing

### Requirement: UI MUST allow users to submit repository analyses
The UI MUST provide an Analyze Repo page or panel with fields for `repo_url`, `ref`, feature/questions input, depth, language, and analyzer mode.

#### Scenario: User submits a repository for analysis
- **WHEN** the user fills the analysis form and submits it
- **THEN** the UI calls the Repo Library ingestion API
- **AND** shows the created analysis run and its current status

### Requirement: UI MUST display repository list and detail views
The UI MUST provide a repository list view and a repository detail view.

The detail view MUST include repository metadata, recent snapshots, analysis runs, extracted cards, and access to the generated report.

#### Scenario: User inspects repository detail
- **WHEN** the user opens an analyzed repository from the list
- **THEN** the UI shows repository metadata and recent snapshots
- **AND** renders extracted cards and report access for the selected snapshot

### Requirement: UI MUST support pattern search
The UI MUST provide a Pattern Search page for semantic Repo Library queries.

#### Scenario: User performs a semantic search
- **WHEN** the user enters a natural-language pattern query and submits it
- **THEN** the UI displays ranked search results with repository context
- **AND** allows navigation from a result to the related repository detail

