## ADDED Requirements

### Requirement: CLI runtime defaults MUST migrate away from vibe-tree-prefixed identifiers
CLI runtime defaults MUST use the current product runtime prefix for managed config roots, environment variables, and generated runtime metadata.

Where older `vibe-tree`-prefixed identifiers already exist in user environments, the runtime MUST preserve compatibility through migration or fallback lookup during the rename transition.

#### Scenario: Runtime starts after rename on a fresh machine
- **WHEN** the system launches a CLI runtime in a fresh environment
- **THEN** managed config roots and runtime metadata use the current runtime prefix

#### Scenario: Runtime starts after rename on an upgraded machine
- **WHEN** the system launches a CLI runtime on a machine that already has old `vibe-tree`-prefixed state
- **THEN** the runtime can still resolve the required state through migration or compatibility lookup
