## ADDED Requirements

### Requirement: Orchestration AI runs MUST use CLI runtime by default
When the system creates a master planning run, agent run, or synthesis run for an orchestration, any AI-capable run MUST resolve through CLI runtime by default.

Helper SDK execution MUST NOT be selected for those runs unless the orchestration explicitly invokes a helper-only utility.

#### Scenario: Orchestration starts with CLI master planning
- **WHEN** a user submits a new orchestration goal
- **THEN** the initial master planning run starts through CLI runtime by default
- **AND** the orchestration remains prompt-first without requiring a pre-created DAG

#### Scenario: Later round agent and synthesis runs stay on CLI runtime
- **WHEN** an orchestration continues into a later round after prior results
- **THEN** newly created agent runs and the round synthesis run use CLI runtime by default
- **AND** helper SDK execution is not chosen for those primary runs

### Requirement: Orchestration detail MUST expose runtime metadata for each run
Each orchestration agent run and synthesis step MUST persist and return runtime metadata sufficient for later inspection.

That metadata MUST include at least:

- runtime kind
- CLI family when applicable
- artifact directory or derived artifact references
- last known runtime session reference when applicable

#### Scenario: Detail response includes runtime metadata
- **WHEN** a client requests orchestration detail after one or more runs have executed
- **THEN** each returned agent run includes its runtime metadata and artifact references
- **AND** clients can correlate those fields with shared execution logs and workspace information
