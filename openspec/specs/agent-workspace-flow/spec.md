# agent-workspace-flow Specification

## Purpose
Agent Workspace Flow 定义 agent 在 read_only/shared_workspace/git_worktree 三种工作区模式下的分配、回退、产物与代码变更摘要规则。
## Requirements
### Requirement: Agent runs MUST declare workspace intent and workspace mode
Each `agent_run` MUST declare an execution intent and workspace mode.

The supported intents MUST distinguish at least analysis, code modification, and verification work.

The supported workspace modes MUST distinguish at least `read_only`, `shared_workspace`, and `git_worktree`.

#### Scenario: Modify agent uses isolated workspace in Git repo
- **WHEN** the master creates an agent run intended to modify code inside a Git repository
- **THEN** the agent run is assigned a `git_worktree`-style isolated workspace by default
- **AND** the workspace assignment is persisted with the agent run

#### Scenario: Non-Git workspace degrades explicitly
- **WHEN** a code-modifying agent run targets a directory without Git worktree support
- **THEN** the system records a fallback workspace mode
- **AND** the fallback reason is persisted for later inspection

### Requirement: Agent runs MUST persist code-oriented workspace metadata
For agent runs that use branch or worktree isolation, the system MUST persist the relevant Git-oriented metadata needed for review and follow-up work.

That metadata MUST include any applicable worktree path, branch name, base branch or base revision reference, and workspace strategy.

#### Scenario: Agent run exposes branch/worktree references
- **WHEN** a modifying agent run completes in an isolated workspace
- **THEN** the persisted agent run data includes its worktree path and branch reference
- **AND** clients can retrieve those references in orchestration detail

### Requirement: Agent runs MUST publish artifact and code-change summaries
Each agent run MUST be able to persist one or more artifacts describing what it produced.

For code-modifying runs, the system MUST persist whether code changed and a human-readable code-change summary.

For non-modifying runs, the system MUST still persist summary artifacts describing the analysis or verification result.

#### Scenario: Modifying agent reports code changes
- **WHEN** an agent run edits repository files
- **THEN** the agent run is marked as having modified code
- **AND** the orchestration detail includes a code-change summary artifact for that run

#### Scenario: Analysis-only agent reports non-code artifact
- **WHEN** an agent run performs repository analysis without editing files
- **THEN** the agent run is marked as not having modified code
- **AND** the orchestration detail includes an analysis summary artifact for that run

### Requirement: Synthesis MUST consume workspace artifacts for merge-oriented reporting
When a round synthesis step is executed, it MUST consume the artifacts and workspace metadata produced by that round's agent runs.

The synthesis output MUST report enough information for a human to understand what changed, where the work lives, and what next project-development action is recommended.

#### Scenario: Synthesis summarizes parallel code branches
- **WHEN** two modifying agent runs complete in the same round
- **THEN** the synthesis output includes both runs' workspace references and code-change summaries
- **AND** the synthesis states the recommended next step for review, merge, or follow-up work

### Requirement: CLI agent runs MUST persist standardized runtime artifact references
For agent runs executed through CLI runtime, the system MUST persist the artifact directory and any structured runtime outputs needed for review, retry, and synthesis.

#### Scenario: Modifying CLI agent persists artifact directory
- **WHEN** a CLI-backed agent run completes after editing repository files
- **THEN** the persisted agent run data includes the artifact directory reference together with its workspace metadata

#### Scenario: Analysis-only CLI agent still persists artifacts
- **WHEN** a CLI-backed agent run completes without editing files
- **THEN** the persisted agent run data still includes the artifact directory reference

### Requirement: Synthesis MUST consume CLI runtime summaries in addition to workspace metadata
When a round synthesis step is executed, it MUST consume both workspace metadata and the standardized CLI runtime summaries produced by that round's agent runs.

#### Scenario: Synthesis uses CLI summaries from parallel runs
- **WHEN** two CLI-backed agent runs complete in the same round
- **THEN** the synthesis input includes each run's runtime summary and workspace reference

