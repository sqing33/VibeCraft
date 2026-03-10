# UI (delta): auto-detect-thinking-translation

## MODIFIED Requirements

### Requirement: Basic settings tab can configure thinking translation

The `基本设置` tab MUST provide a `思考过程翻译` settings section.

The section MUST contain exactly these configurable fields:
- `翻译模型`: a selectable SDK runtime model

The section MUST explain that the system automatically decides whether the model's thinking content needs to be translated into Chinese.

If no SDK runtime model exists, the UI MUST disable the translation configuration field and guide the user to configure a translation model first in the `模型设置` tab.

Saving the form MUST call `PUT /api/v1/settings/basic` and show success or failure feedback.

#### Scenario: Save thinking translation settings

- **WHEN** user selects a translation model and clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/basic`
- **AND** the UI shows a success toast on success
