## MODIFIED Requirements

### Requirement: Repo Library search results MUST be traceable
Each search result MUST preserve a reference back to its repository, analysis result, and related card or chunk so users can inspect the source context.

#### Scenario: User opens a result from pattern search
- **WHEN** the UI selects a returned search result
- **THEN** the backend-provided payload includes repository and analysis-result identifiers
- **AND** the UI can navigate to repository detail or card detail without guessing source context

