## ADDED Requirements

### Requirement: Top-level workspace routes MUST render inside a shared app shell
The application MUST render `#/chat`, `#/orchestrations`, `#/orchestrations/:id`, `#/repo-library/repositories`, `#/repo-library/repositories/:id`, and `#/repo-library/pattern-search` inside a single shared workspace shell.

The shared shell MUST keep the top navigation entry group and the bottom status-and-utility group mounted while switching among these routes.

Only the lane-specific sidebar body and the right content panel MAY change when the active route changes.

#### Scenario: User switches product lanes
- **WHEN** the user switches from Chat to Github 知识库
- **THEN** the left rail frame and the right bordered workspace frame remain in place
- **AND** only the active navigation state, sidebar body, and right content outlet update

#### Scenario: User opens a detail route inside a lane
- **WHEN** the user navigates from a top-level lane page to a lane detail page such as a repository detail or orchestration detail route
- **THEN** the shared workspace shell remains mounted
- **AND** the lane-specific sidebar continues to stay visible beside the new right-panel content

### Requirement: Shared workspace routes MUST preserve immersive split styling
The shared shell MUST render the left rail as part of the page surface instead of as a separate rounded bordered card.

The shared shell MUST render the right content region as a distinct bordered panel with `10px` corner radius.

The outer workspace container MUST use `5px` padding around the split layout.

#### Scenario: User views any shared workspace route
- **WHEN** a shared workspace route is rendered
- **THEN** the left rail visually blends into the page background
- **AND** the right content region remains enclosed by a bordered panel
- **AND** the outer workspace gutter uses `5px` padding
