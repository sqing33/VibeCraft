## MODIFIED Requirements

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

## ADDED Requirements

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
