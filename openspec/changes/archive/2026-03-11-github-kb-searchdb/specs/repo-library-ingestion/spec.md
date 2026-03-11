## ADDED Requirements

### Requirement: Repo Library MUST prepare repository snapshots without Python runtime
The system MUST prepare repository snapshots and artifacts (source checkout, minimal code index) using Go runtime components.

The system MUST NOT require a Python runtime on end-user machines for repository preparation.

#### Scenario: Prepare uses Go-only components
- **WHEN** a repository analysis run starts
- **THEN** the backend prepares the snapshot source tree and `code_index.json` using Go components
- **AND** the process does not invoke `services/repo-analyzer` or Python binaries

### Requirement: Repo Library snapshot source MUST be immutable after preparation
After a snapshot source tree is prepared, the system MUST treat it as immutable.

Re-running an analysis MUST create a new snapshot directory instead of overwriting an existing snapshot source tree.

#### Scenario: Re-analysis creates a new snapshot
- **WHEN** a user submits another analysis for the same repository
- **THEN** the system creates a new snapshot id and storage directory
- **AND** the previous snapshot source remains unchanged

### Requirement: Repo preparation MUST support GitHub ZIP download with git fallback
The system MUST attempt to fetch the repository source via GitHub ZIP archive download first.

If ZIP download fails and a git executable is available, the system MUST fallback to a shallow git clone strategy.

#### Scenario: ZIP fetch fails and git fallback succeeds
- **WHEN** ZIP download fails due to network or archive errors
- **THEN** the system attempts a git clone fallback
- **AND** preparation succeeds when git clone is available

