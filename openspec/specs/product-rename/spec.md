# product-rename Specification

## Purpose

定义从 `vibe-tree` 到 `VibeCraft` 的统一品牌名、运行标识与迁移行为。

## Requirements

### Requirement: Product naming MUST be unified as VibeCraft
The system MUST present `VibeCraft` as the primary product name across user-facing surfaces.

This requirement covers:

- root documentation and primary docs entry points
- UI titles, product labels, about text, and settings-visible product references
- desktop shell display name and packaged app naming
- generated reports or prompts that identify the host product to users

#### Scenario: User opens the main project entry points
- **WHEN** the user reads the root README, opens the app, or inspects the desktop shell name
- **THEN** the primary product name is shown as `VibeCraft`
- **AND** the old product name is not used as the default visible brand

### Requirement: Runtime identifiers MUST use vibecraft-prefixed defaults
The system MUST use `vibecraft` as the default runtime identifier for executable names, default configuration paths, data paths, and environment-variable prefixes.

#### Scenario: User inspects runtime defaults
- **WHEN** the user starts the application in a fresh environment
- **THEN** the default config, data, log, executable, and environment-variable surfaces use `vibecraft` as the canonical prefix

### Requirement: Existing vibe-tree local state MUST remain migratable
The system MUST preserve access to existing local state created under `vibe-tree` defaults.

The migration path MUST either read from the old location or migrate old state into the new `vibecraft` location before normal execution depends on it.

#### Scenario: User upgrades with existing local config and data
- **WHEN** the user already has local configuration or database files under old `vibe-tree` paths
- **THEN** the system preserves access to that state under the new product naming
- **AND** the upgrade does not silently discard the existing state
