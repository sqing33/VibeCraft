# thinking-translation (delta): fix-thinking-translation-chunking

## MODIFIED Requirements

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
