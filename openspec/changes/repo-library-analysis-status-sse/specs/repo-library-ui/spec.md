## ADDED Requirements

### Requirement: Repo Library sidebar MUST show analysis status indicators per repository
Repo Library pages MUST display a per-repository status indicator in the left sidebar repository list based on the latest analysis status.

- If the latest analysis status is `queued` or `running`, the UI MUST show a spinning indicator.
- If the latest analysis status is `failed`, the UI MUST show a failure indicator.
- If the latest analysis status is `succeeded`, the UI MUST NOT show an indicator.

#### Scenario: Repository has a running analysis
- **WHEN** the latest analysis status for a repository is `running`
- **THEN** the sidebar list row shows a spinning indicator

#### Scenario: Repository has a failed analysis
- **WHEN** the latest analysis status for a repository is `failed`
- **THEN** the sidebar list row shows a failure indicator

### Requirement: Repo Library UI MUST refresh repository summaries on status stream events
When the UI is on a Repo Library route, it MUST subscribe to the Repo Library analysis status SSE stream.

When a `repo_library.analysis.updated` event is received, the UI MUST refresh repository summaries using the repositories list API.

#### Scenario: UI receives an update event
- **WHEN** the UI receives a `repo_library.analysis.updated` event
- **THEN** it refreshes the repository list and updates sidebar indicators accordingly

