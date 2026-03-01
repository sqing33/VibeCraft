# Dev Hot Reload (Air)

## Purpose

为本地开发提供后端热重载能力：当开发者修改 `backend/` 下的 Go 源码时，daemon 能自动重建并重启，以缩短联调反馈回路。
## Requirements
### Requirement: Air configuration is provided for backend hot reload
The repository SHALL provide an Air configuration file for the Go backend daemon to enable hot reload during local development.

#### Scenario: Air config exists and is scoped to backend
- **WHEN** a developer uses Air from within `backend/`
- **THEN** Air uses a configuration file located at `backend/.air.toml`

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

### Requirement: Developers can disable Air explicitly
Local development tooling SHALL provide a way to disable Air usage explicitly to simplify debugging and reduce dependencies.

#### Scenario: Air disabled via environment variable
- **WHEN** `scripts/dev.sh` is executed with `VIBE_TREE_NO_AIR=1`
- **THEN** the backend is started via `go run ./cmd/vibe-tree-daemon` even if Air is installed

