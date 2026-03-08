## ADDED Requirements

### Requirement: Automatic compaction MUST use an LLM summary when triggered
When automatic compaction is triggered for a session, the system MUST generate a new session summary using an SDK LLM call.

The generated summary MUST be:
- non-empty
- written in Simplified Chinese
- suitable for long-term continuation (captures key facts, constraints, and decisions)

If LLM summarization fails for any reason, the system MUST fall back to a local deterministic compaction summary and continue without failing the turn.

#### Scenario: Automatic compaction uses LLM summary
- **WHEN** estimated context usage exceeds the compaction threshold before a turn
- **THEN** the system generates a new summary via LLM before the provider call for the turn
- **AND** a compaction record is persisted

#### Scenario: LLM summary failure falls back
- **WHEN** automatic compaction is triggered
- **AND** the LLM summarization call fails
- **THEN** the system generates a local compaction summary instead
- **AND** the turn continues without returning an error due to compaction

### Requirement: Manual compaction MUST use the same LLM-first policy
When a user triggers manual compaction for a session, the system MUST run compaction using the same LLM-first policy as automatic compaction, including the same fallback behavior.

#### Scenario: Manual compact triggers LLM summary
- **WHEN** client calls `POST /api/v1/chat/sessions/:id/compact`
- **THEN** the system updates the session summary via an LLM-first compaction
- **AND** a `chat.session.compacted` event is emitted

