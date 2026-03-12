## MODIFIED Requirements

### Requirement: Repo Library detail MUST present a bounded asymmetric knowledge workspace
The repository detail view MUST present its main content as an asymmetric two-region workspace optimized for browsing repository context, cards, and evidence together.

The workspace MUST keep its primary cards inside a bounded desktop-height working area instead of allowing every panel to expand indefinitely with page height.

The workspace MUST split the main reading area into:
- a left region that takes roughly two-fifths of the width
- a right region that takes roughly three-fifths of the width

The left region MUST stack:
- a compact context control panel for analysis selection
- a repository context panel for stack overview and module summaries whose content can scroll vertically when needed

The right region MUST use a single visual container that stacks:
- a lightweight card-selection strip
- a dominant card detail panel for reading the selected card and its evidence

#### Scenario: User opens repository detail
- **WHEN** the user opens an analyzed repository detail page on a desktop-width layout
- **THEN** the page shows a clear left context region and right reading region instead of three equal-feeling columns
- **AND** the four primary cards remain inside one bounded working area
- **AND** the right-side card-selection strip and detail area read as one continuous panel

### Requirement: Repo Library detail MUST move report access behind an explicit action
The repository detail view MUST expose the full markdown report through an explicit `查看报告` action instead of rendering the entire report body inline in the main workspace.

#### Scenario: User opens the full report
- **WHEN** the selected analysis result has `report_markdown`, `report_excerpt`, or a report file path available
- **THEN** the detail page shows a `查看报告` action
- **AND** activating that action opens a dedicated reading surface for the full report content

### Requirement: Repo Library detail MUST use selectors for snapshot and analysis context
The repository detail view MUST present analysis selection as a compact selector inside the top area of the left context region.

The controls MUST remain visually separate from the repository context summary below them.
The page header MUST show the current analysis result's generated time in place of the previous relative activity text.

#### Scenario: User changes snapshot or analysis
- **WHEN** the user selects a different analysis result
- **THEN** the page updates the current repository context, cards, and detail panel based on that selection
- **AND** the selector area remains compact enough not to compete with the repository context summary
- **AND** the header updates the displayed generated time for the currently selected analysis result

