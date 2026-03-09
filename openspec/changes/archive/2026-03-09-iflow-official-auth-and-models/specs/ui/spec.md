## MODIFIED Requirements

### Requirement: Settings MUST expose dedicated iFlow auth and model controls
The `CLI 工具` tab MUST expose iFlow-specific controls instead of reusing the generic shared-model-only flow.

The iFlow card MUST provide:
- auth mode selector
- browser login launcher/status
- browser-auth URL open action
- authorization code submit input
- API key editor with masked saved state
- iFlow model list editor
- iFlow default model selector

#### Scenario: User starts iFlow browser login from settings
- **WHEN** the user clicks the iFlow browser-login action
- **THEN** the frontend starts a daemon-managed auth session
- **AND** shows the real OAuth URL parsed from the iFlow terminal output
- **AND** allows the user to submit the returned authorization code

### Requirement: Chat and Repo Library MUST use iFlow-specific models
When `iFlow CLI` is selected in Chat or Repo Library, the model selector MUST use the iFlow card model list instead of the shared LLM model pool.

#### Scenario: Chat selects iFlow model list
- **WHEN** the user selects `iFlow CLI` on the chat page
- **THEN** the model selector shows only `iflow_models`
- **AND** the default value comes from `iflow_default_model`

#### Scenario: Repo Library selects iFlow model list
- **WHEN** the user selects `iFlow CLI` on the Repo Library analyze form
- **THEN** the model selector shows only `iflow_models`
- **AND** the submitted `model_id` uses the selected iFlow model name
