# UI (delta): ui-productionize

## ADDED Requirements

### Requirement: Production hides demo-only tools by default

In production builds (`import.meta.env.DEV === false`), the UI MUST hide demo-only tools (e.g. oneshot demo execution) by default. In development builds, demo/diagnostic tools MAY be visible to aid debugging.

#### Scenario: Demo tools hidden in production

- **WHEN** the UI is served from `ui/dist/` by the daemon
- **THEN** demo-only actions are not visible by default

#### Scenario: Demo tools visible in development

- **WHEN** the UI is running under Vite dev server (`import.meta.env.DEV === true`)
- **THEN** demo/diagnostic actions MAY be visible

### Requirement: Workflows-first information architecture

The UI MUST provide a primary navigation centered around Workflows. Connection settings and diagnostics MUST be accessible but MUST NOT dominate the default workflow-oriented UI.

#### Scenario: Opening the app lands on Workflows

- **WHEN** user opens the application
- **THEN** the Workflows Kanban is the primary visible content
- **AND** diagnostics/settings are reachable but not the main focus

### Requirement: Settings contains connection and diagnostics

The UI MUST provide a Settings entry (e.g. drawer/modal) that contains daemon URL switching and diagnostics (version/paths/experts). Changing daemon URL MUST persist to local storage and MUST trigger a health check.

#### Scenario: Change daemon URL in Settings

- **WHEN** user updates the daemon URL in Settings and applies the change
- **THEN** the new URL is saved to local storage
- **AND** the UI performs `GET /api/v1/health` against the new URL
- **AND** the UI updates connection status (Health/WS)
