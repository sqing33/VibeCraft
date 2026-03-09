## ADDED Requirements

### Requirement: Orchestration pages MUST preserve cached recent-list and detail state during route switches
The orchestration home route and orchestration detail route MUST preserve cached recent orchestration data across route changes.

The orchestration detail route MUST preserve previously loaded detail content while a new detail refresh is in progress.

#### Scenario: User switches between orchestration home and detail
- **WHEN** the user moves between the orchestration home route and an orchestration detail route
- **THEN** the recent-orchestration sidebar appears immediately from cache when available
- **AND** the right panel does not flash to an empty state during refresh
