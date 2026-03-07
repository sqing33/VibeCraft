# repo-library-ingestion Specification

## Purpose

Repo Library ingestion 定义从 GitHub 公共仓库发起分析、执行 analyzer、保存仓库快照与运行状态的后端能力。

## ADDED Requirements

### Requirement: Repo Library MUST accept repository analysis requests
The system MUST provide an API for submitting a GitHub repository analysis request with at least `repo_url`, `ref`, and one or more `features`.

The system MUST validate that the repository host is `github.com` and reject empty feature lists.

#### Scenario: User submits a valid analysis request
- **WHEN** a client sends `POST /api/v1/repo-library/analyses` with a public GitHub URL, a ref, and one or more features
- **THEN** the system creates a persistent analysis run record
- **AND** returns the created run metadata with an initial non-terminal status

#### Scenario: User submits an invalid repository request
- **WHEN** a client sends a non-GitHub URL or omits features
- **THEN** the system rejects the request with a validation error
- **AND** no analysis run is created

### Requirement: Repo Library MUST persist repository source, snapshot, and analysis metadata
The system MUST persist normalized repository source records, snapshot records, and analysis run records in SQLite.

Each analysis run MUST retain enough metadata to locate its storage root, report output, status, resolved ref, commit SHA, and latest execution log.

#### Scenario: First analysis of a repository
- **WHEN** a repository is analyzed for the first time
- **THEN** the system creates a repository source record
- **AND** creates a snapshot record for the resolved commit
- **AND** links the new analysis run to that snapshot

#### Scenario: Re-analysis of an existing repository
- **WHEN** a repository already exists in Repo Library and a new analysis is submitted
- **THEN** the system reuses the repository source record
- **AND** creates a new snapshot or analysis run as needed for the resolved commit

### Requirement: Repo Library MUST execute analyzer runs asynchronously
The system MUST execute repository analysis as an asynchronous background run instead of blocking the request lifecycle.

Each run MUST transition through stable statuses including queued, running, succeeded, and failed.

#### Scenario: Analysis run starts after creation
- **WHEN** a new analysis run is accepted
- **THEN** the run enters a queued or running status
- **AND** the backend starts the analyzer process outside the HTTP request path

#### Scenario: Analysis run fails
- **WHEN** the analyzer process exits with a non-zero status or required outputs are missing
- **THEN** the analysis run transitions to failed
- **AND** the failure message is persisted for later inspection

### Requirement: Repo Library MUST store analyzer artifacts under application data storage
The system MUST store repository snapshots, analyzer artifacts, reports, and derived search assets under the application data directory instead of the repository working tree.

#### Scenario: Backend prepares storage for a run
- **WHEN** an analysis run begins
- **THEN** the backend creates or reuses a storage root under the application data directory
- **AND** the run metadata points to that storage path
