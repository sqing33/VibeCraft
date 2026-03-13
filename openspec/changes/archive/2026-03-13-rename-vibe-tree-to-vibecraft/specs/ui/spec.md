## ADDED Requirements

### Requirement: UI MUST present VibeCraft as the host product name
The UI MUST use `VibeCraft` as the host product name in top-level branding, settings, and product-level descriptive text.

#### Scenario: User opens the app shell
- **WHEN** the user opens the application shell or settings surfaces
- **THEN** product-level labels identify the host application as `VibeCraft`

## MODIFIED Requirements

### Requirement: Workflows-first information architecture
The UI MUST provide a primary navigation centered around Workflows. Connection settings and diagnostics MUST be accessible but MUST NOT dominate the default workflow-oriented UI. Product-level chrome and labels MUST consistently use the current host product name.

#### Scenario: Opening the app lands on Workflows

- **WHEN** user opens the application
- **THEN** the Workflows Kanban is the primary visible content
- **AND** diagnostics/settings are reachable but not the main focus
- **AND** product-level branding uses the current host product name

### Requirement: Settings contains connection and diagnostics
The UI MUST provide a Settings entry (e.g. drawer/modal) that contains daemon URL switching and diagnostics (version/paths/experts). Changing daemon URL MUST persist to local storage and MUST trigger a health check. Any product-level path labels or diagnostics copy shown in Settings MUST reflect the current runtime naming and default paths.

#### Scenario: Change daemon URL in Settings

- **WHEN** user updates the daemon URL in Settings and applies the change
- **THEN** the new URL is saved to local storage
- **AND** the UI performs `GET /api/v1/health` against the new URL
- **AND** the UI updates connection status (Health/WS)

#### Scenario: Settings shows current runtime defaults

- **WHEN** the user opens Settings diagnostics after the product rename
- **THEN** the displayed default product paths and labels reflect the current runtime naming
