# cli-chat-thinking (delta): fix-thinking-translation-chunking

## ADDED Requirements

### Requirement: Codex app-server text streaming MUST suppress legacy duplicate deltas
When Codex app-server exposes structured `item/*` text deltas for assistant, reasoning, or plan content, the system MUST treat those structured notifications as the canonical visible stream for that semantic content.

The system MUST NOT append compatible legacy `codex/event/*` text deltas for the same semantic stream into the visible answer/thinking timeline once the structured transport is available for that connection.

#### Scenario: Structured assistant deltas suppress legacy duplicate assistant text
- **WHEN** a Codex app-server connection emits `item/agentMessage/delta` notifications for a turn
- **THEN** the system uses those deltas for visible assistant streaming
- **AND** matching legacy assistant text notifications do not create duplicate visible answer characters

#### Scenario: Structured reasoning deltas suppress legacy duplicate reasoning text
- **WHEN** a Codex app-server connection emits `item/reasoning/summaryTextDelta` or `item/reasoning/textDelta`
- **THEN** the system uses those deltas for visible thinking streaming
- **AND** matching legacy reasoning text notifications do not create duplicate visible thinking characters
