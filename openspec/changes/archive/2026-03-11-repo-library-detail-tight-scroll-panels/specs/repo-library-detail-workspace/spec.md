## MODIFIED Requirements

### Requirement: Repo Library detail MUST present a three-column knowledge workspace
The repository detail view MUST present its main content as an asymmetric two-region workspace optimized for browsing repository context, cards, and evidence together.

The workspace MUST keep its primary cards inside a bounded desktop-height working area instead of allowing every panel to expand indefinitely with page height.

The left region MUST stack:
- a compact context control card
- a repository context card whose content can scroll vertically when needed

The right region MUST use a single visual container that stacks:
- a card-selection strip
- a card detail area whose content can scroll vertically when needed

#### Scenario: User opens repository detail
- **WHEN** the user opens an analyzed repository detail page on a desktop-width layout
- **THEN** the four primary cards remain inside one bounded working area
- **AND** the right-side card-selection strip and detail area read as one continuous panel

### Requirement: Repo Library detail MUST use selectors for snapshot and analysis context
The repository detail view MUST present snapshot selection and analysis-run selection as compact selectors inside the top area of the left context region.

The page header MUST show the current snapshot's generated time in place of the previous relative activity text.

#### Scenario: User changes snapshot or analysis
- **WHEN** the user selects a different snapshot or analysis run
- **THEN** the page updates the current repository context, cards, and detail panel based on that selection
- **AND** the header updates the displayed generated time for the currently selected snapshot

### Requirement: Repo Library detail MUST scope scrolling to content panes
The repository detail page MUST keep the overall workspace stable and assign scrolling to dedicated content panes.

The repository context content area MUST scroll vertically when its text exceeds the allocated height.
The card-selection strip MUST support horizontal scrolling for browsing multiple cards.
The card detail content area MUST scroll vertically as a whole when its content exceeds the allocated height.

#### Scenario: User browses many cards and long detail content
- **WHEN** the selected repository has many cards and the selected card has long detail or evidence content
- **THEN** the user can horizontally scroll the card-selection strip without moving the rest of the layout
- **AND** can vertically scroll the repository context area and card detail area independently inside the bounded workspace
