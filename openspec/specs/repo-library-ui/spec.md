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

When a report body is available as markdown, the detail view MUST render it as markdown instead of displaying the raw markdown source as plain preformatted text.

#### Scenario: User inspects repository detail
- **WHEN** the user opens an analyzed repository from the list
- **THEN** the UI shows repository metadata and recent snapshots
- **AND** renders extracted cards and report access for the selected snapshot

#### Scenario: User reads generated markdown report
- **WHEN** `report_markdown` is available for the selected snapshot
- **THEN** the detail page renders headings, lists, and other markdown structure as formatted content
- **AND** does not show the markdown source as a raw `<pre>` block

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

### Requirement: Repo Library detail MUST distinguish AI Chat analysis process from execution log view
When a repository analysis run is linked to a chat session, the detail view MUST present that chat session as the primary analysis process view.

#### Scenario: Analysis run is chat-linked
- **WHEN** the selected analysis has a `chat_session_id`
- **THEN** the detail page shows an explicit action to open the linked chat session
- **AND** the process section does not mislead the user with a generic “no execution log” empty state

### Requirement: Repo Library detail MUST explain explicit sync behavior
The UI MUST explain that follow-up chat replies do not overwrite the formal analysis result until the user explicitly triggers sync.

#### Scenario: User sees sync guidance
- **WHEN** a repository analysis run is linked to a chat session
- **THEN** the detail page explains that only the explicit sync action updates the formal report/cards/search result

