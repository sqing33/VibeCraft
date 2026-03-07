## ADDED Requirements

### Requirement: CLI agent runs MUST persist standardized runtime artifact references
For agent runs executed through CLI runtime, the system MUST persist the artifact directory and any structured runtime outputs needed for review, retry, and synthesis.

Those persisted references MUST support at least the standard CLI contract files and any derived summaries recorded by the daemon.

#### Scenario: Modifying CLI agent persists artifact directory
- **WHEN** a CLI-backed agent run completes after editing repository files
- **THEN** the persisted agent run data includes the artifact directory reference together with its workspace metadata
- **AND** follow-up review flows can locate the generated summary and patch artifacts

#### Scenario: Analysis-only CLI agent still persists artifacts
- **WHEN** a CLI-backed agent run completes without editing files
- **THEN** the persisted agent run data still includes the artifact directory reference
- **AND** the recorded artifacts describe the analysis output rather than a code change

### Requirement: Synthesis MUST consume CLI runtime summaries in addition to workspace metadata
When a round synthesis step is executed, it MUST consume both workspace metadata and the standardized CLI runtime summaries produced by that round's agent runs.

#### Scenario: Synthesis uses CLI summaries from parallel runs
- **WHEN** two CLI-backed agent runs complete in the same round
- **THEN** the synthesis input includes each run's runtime summary and workspace reference
- **AND** the synthesis result can explain what changed, where artifacts live, and what next action is recommended
