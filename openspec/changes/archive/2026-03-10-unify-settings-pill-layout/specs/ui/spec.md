## ADDED Requirements

### Requirement: Settings dialog MUST use unified pill navigation and compact controls
The settings dialog MUST render its tab navigation as pill-shaped controls, including both the tab list container and the active tab indicator.

Primary `Button`, `Input`, and `Select` controls inside settings tabs MUST use a compact height and full-pill rounded shape.

#### Scenario: User switches settings tabs with pill navigation
- **WHEN** the user opens settings
- **THEN** the top tab navigation renders as a pill-shaped segmented control
- **AND** the selected tab also renders with a pill-shaped active state

### Requirement: Settings tabs MUST keep main actions in a fixed footer
Each settings tab MUST keep its primary actions in a footer region that remains visible while the tab content scrolls vertically.

The tab body MUST be the only vertical scroll area for long settings content.

#### Scenario: User scrolls a long settings tab
- **WHEN** the tab content exceeds the available height
- **THEN** only the content area scrolls vertically
- **AND** the main action footer remains fixed at the bottom of the tab panel
