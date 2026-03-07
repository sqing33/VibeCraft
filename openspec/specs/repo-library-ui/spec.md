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

The UI MUST also provide CLI tool and model selection controls for repository analyses, using the same compatible tool/model pairing rules as Chat.

#### Scenario: User submits a repository for analysis
- **WHEN** the user fills the analysis form, selects a CLI tool and optional model, and submits it
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

### Requirement: UI MUST expose the associated analysis chat session
The repository detail UI MUST expose the automated analysis chat session when one exists, and allow the user to open it directly in the Chat UI.

#### Scenario: User opens analysis chat from repository detail
- **WHEN** a repository analysis run has an associated chat session
- **THEN** the repository detail page shows an action to open that chat session
- **AND** the Chat UI opens with that session selected

