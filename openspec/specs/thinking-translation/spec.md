# thinking-translation Specification

## Purpose

Provide configurable Chinese translation for reasoning/thinking content emitted during chat turns, while automatically skipping thinking that is already Chinese-dominant.

## Requirements

### Requirement: System stores thinking translation settings separately from LLM settings

The system MUST persist thinking translation settings under a dedicated basic settings section instead of mixing them into LLM settings.

The thinking translation settings MUST support:
- `model_id`: an existing SDK runtime model id used for the translation request

The system MUST provide `GET /api/v1/settings/basic` and `PUT /api/v1/settings/basic` to read and write this configuration.

If the referenced `model_id` does not exist in the current runtime model settings, the daemon MUST reject the write with HTTP 400.

#### Scenario: Save valid thinking translation settings

- **WHEN** the client sends `PUT /api/v1/settings/basic` with an existing `model_id`
- **THEN** the daemon persists the configuration and returns HTTP 200

#### Scenario: Reject unknown translation model id

- **WHEN** the client sends `PUT /api/v1/settings/basic` with a `model_id` that does not exist in current runtime model settings
- **THEN** the daemon returns HTTP 400

### Requirement: Thinking translation configuration SHALL stay consistent with runtime model settings

When runtime model settings are updated, the system MUST automatically repair or clear the saved thinking translation configuration so it never references a removed translation model id.

#### Scenario: Clear translation settings when translation model is removed

- **WHEN** the user saves runtime model settings and the saved thinking translation `model_id` no longer exists
- **THEN** the system clears the saved thinking translation configuration before persisting config

### Requirement: Chat turns MAY translate reasoning automatically for non-Chinese thinking content

If a chat turn has a saved thinking translation `model_id`, the system MUST evaluate the emitted thinking content and decide automatically whether translation is needed.

If the thinking content is already Chinese-dominant, the system MUST skip translation for that thinking entry.

If the thinking content is not Chinese-dominant, the system MUST invoke the translation model for that thinking entry.

The system MUST continue emitting the original reasoning stream for compatibility and fallback.

#### Scenario: Chinese-dominant thinking skips translation

- **WHEN** a chat turn emits thinking content that is already Chinese-dominant
- **THEN** the system does not invoke the translation model for that thinking entry

#### Scenario: Non-Chinese thinking triggers translation

- **WHEN** a chat turn emits thinking content that is not Chinese-dominant
- **THEN** the system invokes the translation model for that thinking entry

### Requirement: Reasoning translation SHALL be buffered and streamed in order

The system MUST NOT invoke the translation model for every raw reasoning token.

Instead, the system MUST buffer raw reasoning text and translate it in ordered segments. The system MUST flush buffered text when a sentence-like boundary, a configured size threshold, or turn completion is reached.

An idle window MAY flush a pending segment early only when that segment is already publishable. Interleaving runtime activity MUST NOT force translation of undersized single-word, punctuation-only, or otherwise low-context fragments.

The system MUST ensure at most one translation request is in flight for a given turn at a time.

#### Scenario: Flush translation on sentence boundary

- **WHEN** raw reasoning accumulates to a sentence boundary and the buffered content exceeds the minimum threshold
- **THEN** the system sends one ordered translation request for that buffered segment

#### Scenario: Closed short fragment waits for a better chunk

- **WHEN** a thinking segment is interrupted by tool/progress/system activity but the buffered text is still below the minimum publishable threshold
- **THEN** the system keeps that untranslated fragment buffered for the same thinking entry
- **AND** it does not send an immediate low-context translation request for that fragment

#### Scenario: Flush remaining text when turn completes

- **WHEN** the provider finishes a chat turn and untranslated reasoning text remains buffered
- **THEN** the system translates the remaining buffered text before finalizing the turn result

### Requirement: Translation events and fallback MUST be explicit

For translated turns, the system MUST emit `chat.turn.thinking.translation.delta` events as translated reasoning becomes available.

If translation fails for a turn, the system MUST emit `chat.turn.thinking.translation.failed` and the client MUST be able to fall back to the original reasoning text.

The final `chat.turn.completed` payload MUST include the translated reasoning text when available, and MUST include explicit flags indicating whether translation was actually applied and whether translation failed.

#### Scenario: Translation delta arrives during streaming

- **WHEN** a buffered translation request succeeds before turn completion
- **THEN** the system emits one or more `chat.turn.thinking.translation.delta` events in order

#### Scenario: Translation fails and raw reasoning remains available

- **WHEN** the translation request fails for a translated turn
- **THEN** the system emits `chat.turn.thinking.translation.failed`
- **AND** the original reasoning stream remains available to the client for fallback display

#### Scenario: Turn is configured but no translation is needed

- **WHEN** a turn has thinking translation configured but all thinking entries are judged as Chinese-dominant
- **THEN** `thinking_translation_applied` is `false`
- **AND** `translated_reasoning_text` is empty

### Requirement: Structured runtime feed MUST keep translation scoped to thinking
When thinking translation is enabled during a Codex turn, translated reasoning MUST remain scoped to the active thinking entry in the runtime feed.

For structured Codex turns, `chat.turn.thinking.translation.delta` SHOULD include the target thinking `entry_id`. When a thinking segment is closed by interleaving runtime activity, the system MAY defer undersized buffered translation for that entry until that entry reaches a publishable chunk or the turn completes.

The system MUST NOT overwrite answer, tool, plan, or progress entries with translated thinking text, and deferred translation from an older thinking entry MUST NOT be reassigned to a newer thinking entry.

#### Scenario: Thinking translation updates active thinking entry
- **WHEN** translated thinking deltas are emitted during a Codex turn
- **THEN** the frontend updates the active `kind=thinking` entry with translated content metadata
- **AND** the original answer and tool entries remain unchanged

#### Scenario: Short interrupted entry is translated later for the same entry id
- **WHEN** a tool or plan event interrupts an in-progress thinking segment before it reaches a publishable translation chunk
- **THEN** the system keeps that untranslated tail associated with the original thinking entry
- **AND** any later translated delta for that tail still targets the original `entry_id` instead of the next thinking entry

### Requirement: Thinking translation MUST persist with the corresponding timeline entry
When a selected model uses thinking translation during a chat turn, the system MUST persist translated thinking content together with the corresponding persisted thinking timeline entry.

If translation fails, the system MUST persist the failure state so the frontend can deterministically fall back to original thinking content after refresh.

#### Scenario: Translated thinking survives page refresh
- **WHEN** translation has produced Chinese content for a persisted thinking entry
- **THEN** the translated content is stored with that thinking entry in backend state
- **AND** a refreshed client still renders the translated thinking without waiting for a new translation event

#### Scenario: Translation failure is restorable
- **WHEN** thinking translation fails for a running or completed turn
- **THEN** the system persists the failed translation state for that thinking entry or turn
- **AND** the frontend falls back to original thinking content consistently after refresh
