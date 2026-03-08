# thinking-translation Specification

## Purpose

Provide configurable Chinese translation for reasoning/thinking content emitted by selected AI models during chat turns.

## Requirements

### Requirement: System stores thinking translation settings separately from LLM settings

The system MUST persist thinking translation settings under a dedicated basic settings section instead of mixing them into LLM settings.

The thinking translation settings MUST support:
- `source_id`: an existing LLM Source id used for the translation request
- `model`: a non-empty manually entered translation model name
- `target_model_ids`: one or more existing LLM model ids that should have their reasoning translated

The system MUST provide `GET /api/v1/settings/basic` and `PUT /api/v1/settings/basic` to read and write this configuration.

If any referenced `source_id` or `target_model_ids` does not exist in the current LLM settings, the daemon MUST reject the write with HTTP 400.

#### Scenario: Save valid thinking translation settings

- **WHEN** the client sends `PUT /api/v1/settings/basic` with an existing `source_id`, a non-empty `model`, and one or more existing `target_model_ids`
- **THEN** the daemon persists the configuration and returns HTTP 200

#### Scenario: Reject unknown source id

- **WHEN** the client sends `PUT /api/v1/settings/basic` with a `source_id` that does not exist in current LLM settings
- **THEN** the daemon returns HTTP 400

#### Scenario: Reject unknown target model id

- **WHEN** the client sends `PUT /api/v1/settings/basic` with a `target_model_ids` entry that does not exist in current LLM settings
- **THEN** the daemon returns HTTP 400

### Requirement: Thinking translation configuration SHALL stay consistent with LLM settings

When LLM settings are updated, the system MUST automatically repair or clear the saved thinking translation configuration so it never references removed Sources or removed model ids.

#### Scenario: Clear translation settings when source is removed

- **WHEN** the user saves LLM settings and the saved thinking translation `source_id` no longer exists
- **THEN** the system clears the saved thinking translation configuration before persisting config

#### Scenario: Trim removed target model ids

- **WHEN** the user saves LLM settings and some saved `target_model_ids` no longer exist
- **THEN** the system removes the missing ids from thinking translation settings
- **AND** if no target model id remains, the system clears the saved thinking translation configuration

### Requirement: Chat turns MAY translate reasoning for selected models

If a chat turn resolves to an expert whose `primary_model_id` matches one of the saved `target_model_ids`, the system MUST apply reasoning translation for that turn using the configured translation source and translation model.

The system MUST continue emitting the original reasoning stream for compatibility and fallback.

#### Scenario: Translation applies to selected model

- **WHEN** a chat turn uses an expert whose `primary_model_id` is included in `target_model_ids`
- **THEN** the system invokes the translation model for that turn's reasoning text

#### Scenario: Translation does not apply to unselected model

- **WHEN** a chat turn uses an expert whose `primary_model_id` is not included in `target_model_ids`
- **THEN** the system does not invoke reasoning translation for that turn

### Requirement: Reasoning translation SHALL be buffered and streamed in order

The system MUST NOT invoke the translation model for every raw reasoning token.

Instead, the system MUST buffer raw reasoning text and translate it in ordered segments. The system MUST flush buffered text when a sentence-like boundary, a configured size threshold, an idle window, or turn completion is reached.

The system MUST ensure at most one translation request is in flight for a given turn at a time.

#### Scenario: Flush translation on sentence boundary

- **WHEN** raw reasoning accumulates to a sentence boundary and the buffered content exceeds the minimum threshold
- **THEN** the system sends one ordered translation request for that buffered segment

#### Scenario: Flush remaining text when turn completes

- **WHEN** the provider finishes a chat turn and untranslated reasoning text remains buffered
- **THEN** the system translates the remaining buffered text before finalizing the turn result

### Requirement: Translation events and fallback MUST be explicit

For translated turns, the system MUST emit `chat.turn.thinking.translation.delta` events as translated reasoning becomes available.

If translation fails for a turn, the system MUST emit `chat.turn.thinking.translation.failed` and the client MUST be able to fall back to the original reasoning text.

The final `chat.turn.completed` payload MUST include the translated reasoning text when available, and MUST include explicit flags indicating whether translation was applied and whether translation failed.

#### Scenario: Translation delta arrives during streaming

- **WHEN** a buffered translation request succeeds before turn completion
- **THEN** the system emits one or more `chat.turn.thinking.translation.delta` events in order

#### Scenario: Translation fails and raw reasoning remains available

- **WHEN** the translation request fails for a translated turn
- **THEN** the system emits `chat.turn.thinking.translation.failed`
- **AND** the original reasoning stream remains available to the client for fallback display

### Requirement: Structured runtime feed MUST keep translation scoped to thinking
When thinking translation is enabled during a Codex turn, translated reasoning MUST remain scoped to the active thinking entry in the runtime feed.

The system MUST NOT overwrite answer, tool, plan, or progress entries with translated thinking text.

#### Scenario: Thinking translation updates active thinking entry
- **WHEN** translated thinking deltas are emitted during a Codex turn
- **THEN** the frontend updates the active `kind=thinking` entry with translated content metadata
- **AND** the original answer and tool entries remain unchanged

