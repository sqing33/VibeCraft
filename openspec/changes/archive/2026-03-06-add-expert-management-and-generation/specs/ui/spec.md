## MODIFIED Requirements

### Requirement: Settings uses tab navigation and includes LLM configuration
The UI MUST present the existing System Settings as a tabbed view.
The UI MUST provide at least three tabs:

- `连接与诊断`: contains daemon URL switching and diagnostics (version/paths).
- `模型`: contains LLM Sources / Model Profiles configuration.
- `专家`: contains expert list, expert details, and AI creation workflow.

#### Scenario: User switches to expert settings tab
- **WHEN** user opens System Settings
- **THEN** the UI shows tabs including `连接与诊断`, `模型`, and `专家`
- **AND** switching to `专家` displays expert cards and detail information

### Requirement: Expert tab shows expert metadata and status
The `专家` settings tab MUST display each expert's identity and runtime strategy, including at least name, category, description, managed source, primary model, secondary model, fallback summary, enabled skills, and whether the expert is editable.

#### Scenario: Read expert details in settings
- **WHEN** the expert tab loads successfully
- **THEN** the UI renders a list of experts
- **AND** selecting an expert shows its description, model strategy, skill chips, and prompt summary

### Requirement: Expert tab can create experts through AI conversation
The `专家` settings tab MUST provide an `AI 创建专家` entry that opens a conversation-based creation flow.

#### Scenario: Generate expert draft in modal
- **WHEN** user opens AI 创建专家 and sends a requirement message
- **THEN** the UI calls the expert generation API
- **AND** shows the assistant reply and a structured expert draft preview side-by-side

#### Scenario: Publish generated expert
- **WHEN** user confirms publish on a valid generated draft
- **THEN** the UI saves the expert through `PUT /api/v1/settings/experts`
- **AND** refreshes both the expert settings payload and `GET /api/v1/experts`

### Requirement: Expert tab supports readonly system experts and editable custom experts
The UI MUST distinguish between builtin / llm-model experts and user-managed experts.

#### Scenario: System expert is readonly
- **WHEN** user selects a builtin or llm-model expert
- **THEN** the UI shows its metadata and readonly badges
- **AND** does not show delete actions for that expert

#### Scenario: Custom expert can be toggled or deleted
- **WHEN** user selects a user-managed expert
- **THEN** the UI allows toggling enabled state and deleting the expert
- **AND** saves the updated custom expert list through the expert settings API
