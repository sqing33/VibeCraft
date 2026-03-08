## MODIFIED Requirements

### Requirement: Chat UI MUST render structured runtime layers for Codex CLI turns
The chat page MUST render Codex CLI runtime activity as layered UI blocks instead of a single pending assistant bubble containing mixed content.

The UI MUST support distinct presentation for progress/system messages, thinking, tool execution, plans, user questions, and the final answer.

#### Scenario: Active Codex turn shows layered runtime entries
- **WHEN** the frontend receives `chat.turn.event` entries for an active Codex turn
- **THEN** the chat page renders each entry according to its `kind` with a dedicated style
- **AND** the final answer remains visually primary

#### Scenario: Completed turn keeps process details attached
- **WHEN** a Codex turn completes and the final assistant message is appended
- **THEN** the frontend retains the completed structured runtime feed as a collapsible detail area beneath the corresponding assistant message
- **AND** historical messages without feed data continue to render normally
