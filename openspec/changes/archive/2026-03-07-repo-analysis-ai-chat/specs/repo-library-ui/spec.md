## MODIFIED Requirements

### Requirement: UI MUST allow users to submit repository analyses
The UI MUST provide an Analyze Repo page or panel with fields for `repo_url`, `ref`, feature/questions input, depth, language, and analyzer mode.

The UI MUST also provide CLI tool and model selection controls for repository analyses, using the same compatible tool/model pairing rules as Chat.

#### Scenario: User submits a repository for analysis
- **WHEN** the user fills the analysis form, selects a CLI tool and optional model, and submits it
- **THEN** the UI calls the Repo Library ingestion API
- **AND** shows the created analysis run and its current status

## ADDED Requirements

### Requirement: UI MUST expose the associated analysis chat session
The repository detail UI MUST expose the automated analysis chat session when one exists, and allow the user to open it directly in the Chat UI.

#### Scenario: User opens analysis chat from repository detail
- **WHEN** a repository analysis run has an associated chat session
- **THEN** the repository detail page shows an action to open that chat session
- **AND** the Chat UI opens with that session selected
