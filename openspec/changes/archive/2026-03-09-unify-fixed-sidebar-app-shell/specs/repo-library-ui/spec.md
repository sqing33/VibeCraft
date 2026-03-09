## ADDED Requirements

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
