## MODIFIED Requirements

### Requirement: Dev script prefers Air and falls back safely
The default local development script SHALL start the backend using Air when `air` is available in `PATH`, and SHALL attempt to install Air automatically when it is not available (unless explicitly disabled), and SHALL fall back to the previous `go run` behavior if Air cannot be used.

#### Scenario: Air is installed
- **WHEN** `scripts/dev.sh` is executed and `air` is available in `PATH`
- **THEN** the backend is started via Air

#### Scenario: Air is not installed but auto-install succeeds
- **WHEN** `scripts/dev.sh` is executed and `air` is not available in `PATH`
- **AND** `go install github.com/air-verse/air@latest` succeeds
- **THEN** the backend is started via Air

#### Scenario: Air is not installed and auto-install fails
- **WHEN** `scripts/dev.sh` is executed and `air` is not available in `PATH`
- **AND** Air auto-install fails (missing `go` or install error)
- **THEN** the backend is started via `go run ./cmd/vibe-tree-daemon`
