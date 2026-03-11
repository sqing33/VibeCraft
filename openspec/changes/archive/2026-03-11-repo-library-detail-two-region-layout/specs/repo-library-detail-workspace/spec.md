## MODIFIED Requirements

### Requirement: Repo Library detail MUST present a three-column knowledge workspace
The repository detail view MUST present its main content as an asymmetric two-region workspace optimized for browsing repository context, cards, and evidence together.

The workspace MUST split the main reading area into:
- a left region that takes roughly two-fifths of the width
- a right region that takes roughly three-fifths of the width

The left region MUST stack:
- a compact context control panel for snapshot and analysis selection
- a repository context panel for generated summary, stack overview, and module summaries

The right region MUST stack:
- a lightweight card-selection strip
- a dominant card detail panel for reading the selected card and its evidence

#### Scenario: User opens repository detail
- **WHEN** the user opens an analyzed repository detail page on a desktop-width layout
- **THEN** the page shows a clear left context region and right reading region instead of three equal-feeling columns
- **AND** the right reading region is visually dominant over the left context region

### Requirement: Repo Library detail MUST use selectors for snapshot and analysis context
The repository detail view MUST present snapshot selection and analysis-run selection as compact selectors inside the top area of the left context region.

The controls MUST remain visually separate from the repository context summary below them.

#### Scenario: User changes snapshot or analysis
- **WHEN** the user selects a different snapshot or analysis run
- **THEN** the page updates the current repository context, cards, and detail panel based on that selection
- **AND** the selector area remains compact enough not to compete with the repository context summary

### Requirement: Repo Library detail MUST scope scrolling to content panes
The repository detail page MUST keep the overall workspace stable and assign scrolling to dedicated pane regions.

The repository context panel in the left region MUST scroll independently when its content exceeds the available height.
The card-selection strip in the right region MUST support horizontal scrolling for browsing multiple cards.
The evidence region inside the right detail panel MUST scroll independently from the card summary above it.

#### Scenario: User browses many cards and long evidence
- **WHEN** the selected repository has many cards and the selected card has long evidence content
- **THEN** the user can horizontally scroll the card-selection strip without moving the left context region
- **AND** can scroll evidence without losing the selected card's title, conclusion, and mechanism
