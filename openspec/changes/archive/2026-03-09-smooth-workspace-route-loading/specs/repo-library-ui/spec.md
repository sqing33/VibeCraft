## ADDED Requirements

### Requirement: Repo Library pages MUST preserve cached list and detail state during route switches
Repo Library list, search, and detail routes MUST preserve cached repository-list data across route changes.

Repo Library detail routes MUST preserve previously loaded detail content while a new repository detail refresh is in progress.

#### Scenario: User switches between Repo Library routes
- **WHEN** the user moves between repository list, search, and detail routes
- **THEN** the left repository list appears immediately from cache when available
- **AND** the right panel does not flash to an empty state during refresh
