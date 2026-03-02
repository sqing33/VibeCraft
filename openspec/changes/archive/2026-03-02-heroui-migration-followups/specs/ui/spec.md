# UI (delta): heroui-migration-followups

## ADDED Requirements

### Requirement: LLM model profiles require a valid Source

When at least one LLM Source exists, the UI MUST ensure each model profile is bound to a non-empty, valid Source ID. The UI MUST prevent saving or testing LLM settings when any model profile has an empty Source selection.

#### Scenario: User cannot clear Source selection

- **WHEN** a model profile has at least one available Source option
- **AND** the user attempts to clear the Source selection in the UI
- **THEN** the UI keeps a non-empty Source selection (either the previous value or a default)

#### Scenario: Saving is blocked when Source is missing

- **WHEN** the user clicks Save while any model profile has an empty Source selection
- **THEN** the UI shows an error toast describing the missing Source
- **AND** does not submit the settings update request

