## MODIFIED Requirements

### Requirement: Chat model selectors MUST display the selected model label
The Chat page's tool-first model selectors MUST visibly display the currently selected model label whenever the selected key matches an available model option.

#### Scenario: New-session model selector shows selected label
- **WHEN** user selects a model for a new chat session
- **THEN** the model select control shows that model label in the collapsed field

#### Scenario: Composer model selector shows current session model
- **WHEN** an active session already has a selected tool/model combination
- **THEN** the composer model select shows the matching label instead of an empty field
