## ADDED Requirements

### Requirement: Workflow-facing path defaults MUST use the current runtime prefix
Workflow-facing documentation, diagnostics, and default execution path references MUST use the current runtime prefix instead of legacy project naming.

#### Scenario: User checks workflow-related runtime paths
- **WHEN** the user inspects workflow diagnostics, logs, or documentation after the rename
- **THEN** the referenced default workflow log and data paths use the current runtime prefix
