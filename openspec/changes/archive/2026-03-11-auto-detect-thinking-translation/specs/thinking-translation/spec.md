# Thinking Translation (delta): auto-detect-thinking-translation

## MODIFIED Requirements

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

### Requirement: Thinking translation configuration SHALL stay consistent with LLM settings

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

### Requirement: Translation events and fallback MUST be explicit

For translated turns, the system MUST emit `chat.turn.thinking.translation.delta` events as translated reasoning becomes available.

If translation fails for a turn, the system MUST emit `chat.turn.thinking.translation.failed` and the client MUST be able to fall back to the original reasoning text.

The final `chat.turn.completed` payload MUST include the translated reasoning text when available, and MUST include explicit flags indicating whether translation was actually applied and whether translation failed.

#### Scenario: Turn is configured but no translation is needed

- **WHEN** a turn has thinking translation configured but all thinking entries are judged as Chinese-dominant
- **THEN** `thinking_translation_applied` is `false`
- **AND** `translated_reasoning_text` is empty
