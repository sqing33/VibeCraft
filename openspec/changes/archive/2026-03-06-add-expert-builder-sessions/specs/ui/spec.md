## MODIFIED Requirements

### Requirement: Expert tab can create experts through AI conversation
The `专家` settings tab MUST provide an `AI 创建专家` entry that opens a conversation-based creation flow.

The creation flow MUST be session-based rather than stateless. It MUST support loading prior messages, continuing the conversation, and previewing historical draft snapshots.

#### Scenario: Create new builder session from expert tab
- **WHEN** user clicks `AI 创建专家`
- **THEN** the UI allows selecting a configured builder model and creates a new builder session
- **AND** the modal displays an empty conversation plus current draft area

#### Scenario: Continue long conversation
- **WHEN** user sends multiple follow-up messages in the same builder session
- **THEN** the UI appends the full history in order
- **AND** each round updates the latest draft preview instead of replacing the whole session

#### Scenario: Continue optimizing an existing expert
- **WHEN** user chooses to optimize a published expert
- **THEN** the UI loads the related builder session if one exists
- **AND** allows continuing the conversation using the saved history

#### Scenario: Inspect historical draft snapshots
- **WHEN** user opens a builder session with multiple snapshots
- **THEN** the UI shows a snapshot list with version and time
- **AND** selecting a snapshot updates the draft preview
