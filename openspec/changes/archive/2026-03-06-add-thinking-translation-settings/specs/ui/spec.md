# UI (delta): add-thinking-translation-settings

## MODIFIED Requirements

### Requirement: Settings uses tab navigation and includes LLM configuration

The UI MUST present the existing System Settings as a tabbed view.
The UI MUST provide at least four tabs:

- `基本设置`: contains thinking translation configuration.
- `连接与诊断`: contains daemon URL switching and diagnostics (version/paths).
- `模型`: contains LLM Sources / Model Profiles configuration.
- `专家`: contains expert list, expert details, and AI creation workflow.

#### Scenario: User switches settings tabs

- **WHEN** user opens System Settings
- **THEN** the UI shows multiple tabs including `基本设置`, `连接与诊断`, `模型`, and `专家`
- **AND** switching tabs updates the visible settings content

## ADDED Requirements

### Requirement: Basic settings tab can configure thinking translation

The `基本设置` tab MUST provide a `思考过程翻译` settings section.

The section MUST contain exactly these configurable fields:
- `API 源`: selects an existing LLM Source
- `翻译模型`: a manually entered model string
- `需要翻译的 AI 模型`: a multi-select list populated from all configured LLM models

If no LLM Source exists, the UI MUST disable the translation configuration fields and guide the user to configure sources first in the `模型` tab.

If LLM models do not exist yet, the UI MUST disable the target model selector.

Saving the form MUST call `PUT /api/v1/settings/basic` and show success or failure feedback.

#### Scenario: Save thinking translation settings

- **WHEN** user selects a source, enters a translation model, selects one or more target AI models, and clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/basic`
- **AND** the UI shows a success toast on success

#### Scenario: Basic settings disabled before model configuration

- **WHEN** the user opens `基本设置` before configuring any LLM Source
- **THEN** the UI disables the thinking translation fields
- **AND** the UI shows guidance to configure API Sources in the `模型` tab first

### Requirement: UI SHALL prefer translated reasoning for translated turns

When a translated reasoning stream is available for the current turn, the UI MUST display the translated Chinese reasoning instead of the original reasoning text.

If translation is enabled for the turn but translated text has not arrived yet, the UI SHOULD show a Chinese loading hint rather than immediately rendering the original English reasoning.

If translation fails for the turn, the UI MUST fall back to displaying the original reasoning text.

#### Scenario: Show translated reasoning only

- **WHEN** the current chat turn receives translated reasoning deltas successfully
- **THEN** the reasoning UI displays the translated Chinese text
- **AND** it does not display the original English reasoning at the same time

#### Scenario: Fallback to original reasoning on translation failure

- **WHEN** the current chat turn receives a `chat.turn.thinking.translation.failed` event
- **THEN** the reasoning UI falls back to displaying the original reasoning text for that turn
