## ADDED Requirements

### Requirement: Chat UI MUST render CLI mid-turn feedback
The Chat UI MUST render CLI-generated mid-turn assistant deltas as they arrive, and it MUST render available thinking/progress events without waiting for turn completion.

#### Scenario: Assistant bubble grows during CLI turn
- **WHEN** CLI assistant deltas are received through WebSocket
- **THEN** the pending assistant bubble updates incrementally before completion

#### Scenario: Thinking or progress panel updates during CLI turn
- **WHEN** CLI thinking/progress events are received
- **THEN** the UI updates the mid-turn thinking/progress area before the final answer completes
