## MODIFIED Requirements

### Requirement: Repo Library MUST accept repository analysis requests
The system MUST provide an API for submitting a GitHub repository analysis request with at least `repo_url`, `ref`, and one or more `features`.

The system MUST validate that the repository host is `github.com` and reject empty feature lists.

The request MUST additionally support selecting a CLI analysis tool and an optional model identifier compatible with that tool's protocol family.

#### Scenario: User submits a valid analysis request
- **WHEN** a client sends `POST /api/v1/repo-library/analyses` with a public GitHub URL, a ref, one or more features, and an optional CLI tool/model selection
- **THEN** the system creates a persistent analysis run record
- **AND** returns the created run metadata with an initial non-terminal status

#### Scenario: User submits an invalid repository request
- **WHEN** a client sends a non-GitHub URL or omits features
- **THEN** the system rejects the request with a validation error
- **AND** no analysis run is created

### Requirement: Repo Library MUST execute analyzer runs asynchronously
The system MUST execute repository analysis as an asynchronous background run instead of blocking the request lifecycle.

Each run MUST transition through stable statuses including queued, running, succeeded, and failed.

The asynchronous execution chain MUST include repository preparation, automated AI chat analysis, markdown report persistence, card extraction, and search-index refresh.

#### Scenario: Analysis run starts after creation
- **WHEN** a new analysis run is accepted
- **THEN** the run enters a queued or running status
- **AND** the backend starts the repository preparation and AI chat analysis chain outside the HTTP request path

#### Scenario: Analysis run fails after chat creation
- **WHEN** repository preparation succeeds but the automated AI chat or post-processing step fails
- **THEN** the analysis run transitions to failed
- **AND** the failure message is persisted for later inspection
- **AND** any created chat session remains available for inspection or manual continuation
