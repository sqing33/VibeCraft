## ADDED Requirements

### Requirement: Repo Library detail MUST present a three-column knowledge workspace
The repository detail view MUST present its main content as a three-column workspace optimized for browsing repository context, cards, and evidence together.

The left column MUST present repository context controls and summaries, the middle column MUST present the card list, and the right column MUST present card detail and evidence.

#### Scenario: User opens repository detail
- **WHEN** the user opens an analyzed repository detail page on a desktop-width layout
- **THEN** the page shows three persistent columns for context, card list, and card detail
- **AND** the user can inspect cards and evidence without scrolling the entire page

### Requirement: Repo Library detail MUST move report access behind an explicit action
The repository detail view MUST expose the full markdown report through an explicit `查看报告` action instead of rendering the entire report body inline in the main workspace.

#### Scenario: User opens the full report
- **WHEN** the selected snapshot has `report_markdown`, `report_excerpt`, or a report file path available
- **THEN** the detail page shows a `查看报告` action
- **AND** activating that action opens a dedicated reading surface for the full report content

### Requirement: Repo Library detail MUST use selectors for snapshot and analysis context
The repository detail view MUST present snapshot selection and analysis-run selection as compact selectors rather than a vertically expanded list of cards.

#### Scenario: User changes snapshot or analysis
- **WHEN** the user selects a different snapshot or analysis run
- **THEN** the page updates the current repository context, cards, and detail panel based on that selection
- **AND** the control area remains compact enough to coexist with the rest of the three-column workspace

### Requirement: Repo Library detail MUST scope scrolling to content panes
The repository detail page MUST keep the overall workspace stable and assign scrolling to dedicated pane regions.

The middle card-list pane MUST scroll independently, and the evidence region inside the right detail pane MUST scroll independently from the card summary above it.

#### Scenario: User browses many cards and long evidence
- **WHEN** the selected repository has many cards and the selected card has long evidence content
- **THEN** the user can scroll the card list without moving the left or right pane headers
- **AND** can scroll evidence without losing the selected card's title, conclusion, and mechanism
