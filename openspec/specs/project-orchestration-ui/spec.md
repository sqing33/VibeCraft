# project-orchestration-ui Specification

## Purpose
Project Orchestration UI 定义新的主入口、轮次视图、并行 agent 卡片、详情面板与 Legacy Workflows 隐藏兼容策略。
## Requirements
### Requirement: Orchestrations MUST be the primary UI entry and Legacy Workflows MUST remain hidden-but-compatible
The UI MUST expose `Orchestrations` as the primary project-development entry.

The existing workflow surface MUST remain available for compatibility, but it MUST NOT remain a visible primary navigation entry.

The product MUST preserve a compatibility path for existing workflow links while steering all default entry behavior toward orchestration routes.

#### Scenario: Home opens orchestration surface
- **WHEN** a user opens the main application entry
- **THEN** the primary visible route is the orchestration surface
- **AND** the application does not default back to the legacy workflow list

#### Scenario: Existing workflow link still works
- **WHEN** a user opens an existing workflow-oriented route or bookmark
- **THEN** the app still resolves that route to the legacy workflow surface
- **AND** the UI treats that surface as a hidden legacy compatibility path rather than as the main product path

### Requirement: The orchestration page MUST provide a top goal input area
The orchestration page MUST provide a top-level input area for entering a natural-language goal, along with the minimal context needed to start work (for example workspace path or repository context).

Submitting that input MUST create and start an orchestration.

#### Scenario: User submits a new orchestration goal
- **WHEN** the user enters a goal in the top input area and submits it
- **THEN** the UI starts a new orchestration request
- **AND** the UI transitions to the created orchestration detail view

### Requirement: Each round MUST render sibling agent cards in one row
The orchestration detail UI MUST render rounds as ordered sections, and all sibling agent runs created for the same round MUST appear together in one row or lane.

Each agent card MUST display at least:
- role
- task goal
- current status
- output summary
- log availability/status
- whether code was modified

#### Scenario: Parallel agents appear in the same round row
- **WHEN** round 1 contains three sibling agent runs
- **THEN** the UI shows those three agent cards within the same round row
- **AND** the user can compare their roles and statuses side by side

### Requirement: The UI MUST provide a detail panel for logs and artifacts
Selecting an agent run or synthesis step MUST open a detail panel that can show its logs, execution status, summaries, and associated artifacts or code-change information.

#### Scenario: User inspects an agent run
- **WHEN** the user selects an agent card that has execution history
- **THEN** the detail panel shows the agent's logs and summaries
- **AND** the panel indicates whether the agent modified code and what artifacts are available

#### Scenario: User inspects synthesis output
- **WHEN** the user selects a synthesis step
- **THEN** the detail panel shows the synthesis summary and next-step recommendation

### Requirement: The orchestration UI MUST expose control actions
The orchestration UI MUST expose control actions appropriate to the selected state, including canceling a running orchestration, retrying a failed agent run, and continuing an orchestration after synthesis or recoverable pause.

#### Scenario: Retry failed agent from detail view
- **WHEN** an agent run is in a retryable terminal state
- **THEN** the UI offers a retry action for that agent run
- **AND** the round/orchestration view updates when the retry starts

#### Scenario: Continue orchestration after synthesis
- **WHEN** an orchestration reaches a state where further work is allowed
- **THEN** the UI offers a continue action
- **AND** the orchestration view reflects the next planning or round transition after the action succeeds

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

### Requirement: Orchestration pages MUST preserve cached recent-list and detail state during route switches
The orchestration home route and orchestration detail route MUST preserve cached recent orchestration data across route changes.

The orchestration detail route MUST preserve previously loaded detail content while a new detail refresh is in progress.

#### Scenario: User switches between orchestration home and detail
- **WHEN** the user moves between the orchestration home route and an orchestration detail route
- **THEN** the recent-orchestration sidebar appears immediately from cache when available
- **AND** the right panel does not flash to an empty state during refresh

