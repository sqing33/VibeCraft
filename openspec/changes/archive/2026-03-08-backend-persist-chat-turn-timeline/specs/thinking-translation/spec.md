## ADDED Requirements

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
