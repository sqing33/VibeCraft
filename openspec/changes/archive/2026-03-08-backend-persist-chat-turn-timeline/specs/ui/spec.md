## ADDED Requirements

### Requirement: Chat UI MUST restore runtime timelines from backend snapshots
The chat page MUST restore completed and running Codex process timelines from backend turn snapshots instead of treating browser runtime state as the source of truth.

The frontend MAY still keep lightweight view state locally, but it MUST derive visible process content from backend `messages` plus persisted `turns` data.

#### Scenario: Refresh during running turn preserves pending timeline
- **WHEN** the user refreshes the chat page while a Codex-backed turn is still running
- **THEN** the page reloads the persisted running turn snapshot from the backend
- **AND** the pending assistant bubble continues showing the already persisted process timeline

#### Scenario: Refresh after completion does not create duplicate assistant bubbles
- **WHEN** the user refreshes after a turn has completed
- **THEN** the page renders one completed assistant message bubble
- **AND** the associated process details come from the persisted completed turn timeline instead of a separate stale pending bubble
