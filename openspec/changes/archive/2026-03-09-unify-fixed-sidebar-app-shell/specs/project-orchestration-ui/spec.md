## ADDED Requirements

### Requirement: Orchestration routes MUST render inside the shared workspace shell
The orchestration list route and orchestration detail route MUST render inside the shared workspace shell.

Within that shell, the middle left sidebar region MUST show recent orchestrations.

The right content panel MUST switch between orchestration creation/list content and orchestration detail content without replacing the shared shell chrome.

#### Scenario: User opens orchestration detail from the recent list
- **WHEN** the user selects a recent orchestration from the left sidebar
- **THEN** the shared workspace shell remains mounted
- **AND** the left sidebar continues to show recent orchestrations
- **AND** the right content panel shows the selected orchestration detail

#### Scenario: User returns to the orchestration home route
- **WHEN** the user navigates from an orchestration detail route back to `#/orchestrations`
- **THEN** the shared workspace shell remains mounted
- **AND** the left sidebar continues to show recent orchestrations
- **AND** the right content panel shows the orchestration home content
