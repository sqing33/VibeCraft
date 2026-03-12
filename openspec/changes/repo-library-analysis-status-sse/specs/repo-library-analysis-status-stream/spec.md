## ADDED Requirements

### Requirement: Repo Library daemon MUST expose an SSE stream for analysis status updates
The daemon MUST expose an HTTP endpoint that streams server-sent events for Repo Library analysis status changes.

The endpoint MUST be available at `GET /api/v1/repo-library/stream` and MUST return `Content-Type: text/event-stream`.

#### Scenario: Client subscribes to the stream
- **WHEN** the client opens an EventSource to `/api/v1/repo-library/stream`
- **THEN** the connection remains open and receives events as analysis status changes occur

### Requirement: The stream MUST emit status update events on analysis lifecycle changes
The daemon MUST emit an SSE event named `repo_library.analysis.updated` when a Repo Library analysis status transitions or is finalized.

The event payload MUST include `repository_id`, `analysis_id`, `status`, and `updated_at` (Unix ms).

#### Scenario: Analysis transitions to running
- **WHEN** an analysis is marked as `running`
- **THEN** the stream emits a `repo_library.analysis.updated` event with status `running`

#### Scenario: Analysis is finalized
- **WHEN** an analysis is finalized as `succeeded` or `failed`
- **THEN** the stream emits a `repo_library.analysis.updated` event with the final status

