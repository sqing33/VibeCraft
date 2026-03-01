# UI (delta): ui-localize-zh-and-theme-toggle

## ADDED Requirements

### Requirement: UI text is Chinese by default

The UI MUST present user-facing copy in Simplified Chinese across the primary workflow path, including navigation labels, action buttons, form labels, status text, empty states, and error prompts.

#### Scenario: User opens workflows page

- **WHEN** user opens the application
- **THEN** primary page titles and action buttons are displayed in Simplified Chinese

#### Scenario: User sees error or empty state

- **WHEN** workflow list or detail page enters error/empty/loading states
- **THEN** the corresponding visible prompts are displayed in Simplified Chinese

### Requirement: UI supports light and dark theme toggle

The UI MUST provide a user-accessible theme switch for light and dark modes. Theme selection MUST be persisted in local storage and restored on next load.

#### Scenario: User toggles theme

- **WHEN** user switches between light and dark theme in UI settings/topbar
- **THEN** the entire UI theme updates immediately
- **AND** selected theme is saved to local storage

#### Scenario: User reloads app after selecting theme

- **WHEN** user reloads the app after previously selecting dark or light theme
- **THEN** the UI restores the saved theme from local storage

### Requirement: Default theme is light

When no persisted theme value exists, the UI MUST initialize with light theme.

#### Scenario: First-time visitor

- **WHEN** local storage does not contain a saved theme key
- **THEN** the UI loads in light theme
