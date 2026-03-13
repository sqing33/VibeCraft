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
- **AND** shows the created analysis result and its current status

### Requirement: UI MUST display repository list and detail views
The UI MUST provide a repository list view and a repository detail view.

The detail view MUST include repository metadata, recent analyses, extracted cards, and access to the generated report.

When a report body is available as markdown, the detail view MUST render it as markdown instead of displaying the raw markdown source as plain preformatted text.

#### Scenario: User inspects repository detail
- **WHEN** the user opens an analyzed repository from the list
- **THEN** the UI shows repository metadata and recent analyses
- **AND** renders extracted cards and report access for the selected analysis result

#### Scenario: User reads generated markdown report
- **WHEN** `report_markdown` is available for the selected analysis result
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
- **WHEN** a repository analysis result has an associated chat session
- **THEN** the repository detail page shows an action to open that chat session
- **AND** the Chat UI opens with that session selected

### Requirement: Repo Library detail MUST distinguish AI Chat analysis process from execution log view
When a repository analysis result is linked to a chat session, the detail view MUST present that chat session as the primary analysis process view.

#### Scenario: Analysis run is chat-linked
- **WHEN** the selected analysis has a `chat_session_id`
- **THEN** the detail page shows an explicit action to open the linked chat session
- **AND** the process section does not mislead the user with a generic “no execution log” empty state

### Requirement: Repo Library detail MUST explain explicit sync behavior
The UI MUST explain that follow-up chat replies do not overwrite the formal analysis result until the user explicitly triggers sync.

#### Scenario: User sees sync guidance
- **WHEN** a repository analysis result is linked to a chat session
- **THEN** the detail page explains that only the explicit sync action updates the formal report/cards/search result

### Requirement: Repo Library routes MUST render inside the shared workspace shell
The repository list page, repository detail page, and pattern search page MUST render inside the shared workspace shell.

Within that shell, the middle left sidebar region MUST show the repository list and a persistent `添加仓库` action.

The right content panel MUST switch among repository list, repository detail, and `知识库检索` content without replacing the shared shell chrome.

#### Scenario: User opens repository detail from the list
- **WHEN** the user selects a repository from the left sidebar list
- **THEN** the shared workspace shell remains mounted
- **AND** the right content panel shows the selected repository detail
- **AND** the left sidebar continues to show the repository list and `添加仓库` action

#### Scenario: User enters pattern search from the shared shell
- **WHEN** the user opens the `知识库检索` route from the repository lane
- **THEN** the shared workspace shell remains mounted
- **AND** the left sidebar continues to show the repository list and `添加仓库` action
- **AND** the right content panel shows the search interface

### Requirement: Pattern search MUST search across all indexed repositories by default
The pattern search page MUST search across all indexed repositories by default.

The repository list shown in the left sidebar on the pattern search page MUST act as navigation to repository detail routes and MUST NOT act as an active search filter.

#### Scenario: User clicks a repository while on pattern search
- **WHEN** the user is on `知识库检索` and clicks a repository in the left sidebar
- **THEN** the click navigates to the selected repository detail page
- **AND** the search scope remains all indexed repositories unless the user submits other explicit query criteria

### Requirement: Repo Library pages MUST preserve cached list and detail state during route switches
Repo Library list, search, and detail routes MUST preserve cached repository-list data across route changes.

Repo Library detail routes MUST preserve previously loaded detail content while a new repository detail refresh is in progress.

#### Scenario: User switches between Repo Library routes
- **WHEN** the user moves between repository list, search, and detail routes
- **THEN** the left repository list appears immediately from cache when available
- **AND** the right panel does not flash to an empty state during refresh

### Requirement: Repo Library MUST be described as the GitHub knowledge base
The UI and primary documentation MUST describe Repo Library as the product's GitHub knowledge base rather than as a generic report list.

#### Scenario: User opens Repo Library entry points
- **WHEN** the user opens Repo Library views or the primary project documentation
- **THEN** Repo Library is explained as a GitHub knowledge base
- **AND** the description makes clear that reports, cards, and evidence are reusable knowledge assets

### Requirement: Repo Library search MUST describe vector-backed retrieval
Repo Library pattern search and related documentation MUST explain that retrieval is backed by local vector indexes plus structured repository artifacts.

#### Scenario: User reads Repo Library search description
- **WHEN** the user opens pattern search surfaces or product documentation about Repo Library
- **THEN** the system explains that search uses local vector-backed retrieval together with structured analysis results
