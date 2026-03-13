## ADDED Requirements

### Requirement: Air configuration is provided for backend hot reload
The repository SHALL provide an Air configuration file for the Go backend daemon to enable hot reload during local development.

#### Scenario: Air config exists and is scoped to backend
- **WHEN** a developer uses Air from within `backend/`
- **THEN** Air uses a configuration file located at `backend/.air.toml`

### Requirement: Dev script prefers Air and falls back safely
The default local development script SHALL start the backend using Air when `air` is available in `PATH`, and SHALL fall back to the previous `go run` behavior when Air is not available.

#### Scenario: Air is installed
- **WHEN** `scripts/dev.sh` is executed and `air` is available in `PATH`
- **THEN** the backend is started via Air

#### Scenario: Air is not installed
- **WHEN** `scripts/dev.sh` is executed and `air` is not available in `PATH`
- **THEN** the backend is started via `go run ./cmd/vibecraft-daemon`

### Requirement: Developers can disable Air explicitly
Local development tooling SHALL provide a way to disable Air usage explicitly to simplify debugging and reduce dependencies.

#### Scenario: Air disabled via environment variable
- **WHEN** `scripts/dev.sh` is executed with `VIBECRAFT_NO_AIR=1`
- **THEN** the backend is started via `go run ./cmd/vibecraft-daemon` even if Air is installed
