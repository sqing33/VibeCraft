## 1. Slice A — Runtime & Persistence Foundation

- [x] 1.1 Add SQLite migrations for `orchestrations`, `orchestration_rounds`, `agent_runs`, `synthesis_steps`, and `artifacts`
- [x] 1.2 Define Go store models and status enums for orchestration, round, agent run, synthesis step, and artifact records
- [x] 1.3 Extend execution persistence so an execution can optionally correlate to `orchestration_id`, `round_id`, and `agent_run_id` without breaking legacy workflow/node queries
- [x] 1.4 Implement store queries for orchestration list/detail, ordered round history, agent run detail, synthesis detail, and recovery of non-terminal orchestration records

## 2. Slice A — Shared Runtime Extraction

- [x] 2.1 Extract reusable execution launch/finalize helpers from legacy workflow start and scheduler paths
- [x] 2.2 Refactor legacy workflow master start and worker scheduling to consume the shared runtime helpers without behavior changes
- [x] 2.3 Extend execution lifecycle events and in-memory state to emit orchestration correlation fields while preserving existing workflow/node consumers
- [x] 2.4 Reuse the global concurrency budget for orchestration scheduling and expose the remaining slot count to orchestration runtime decisions

## 3. Slice B — Orchestration Backend Skeleton

- [x] 3.1 Add orchestration HTTP APIs for create/start (`POST /api/v1/orchestrations`), list, and detail
- [x] 3.2 Add orchestration store/service logic that creates a master planning run directly from user goal input instead of from a pre-created workflow
- [x] 3.3 Define and implement the master planning output contract for round plans (round goal + agent run list), explicitly replacing DAG JSON for the new flow
- [x] 3.4 Materialize the first `round` and its sibling `agent_runs` from the master planning result and persist them before execution begins

## 4. Slice B/C — Round Execution & Synthesis Loop

- [x] 4.1 Implement round-aware scheduling so sibling `agent_runs` can run in parallel within available execution slots
- [x] 4.2 Persist agent run summaries, current/last execution references, and terminal states as executions finish
- [x] 4.3 Add round barrier detection so synthesis only starts after all agent runs in the round are terminal
- [x] 4.4 Implement `synthesis_step` creation with persisted decisions for `complete`, `continue`, and `needs_retry`
- [x] 4.5 Add orchestration controls for `cancel`, `continue`, and `agent_run retry`, using shared execution cancel/retry semantics underneath

## 5. Slice C/D — Workspace & Artifact Flow

- [x] 5.1 Implement workspace strategy resolution for `read_only`, `shared_workspace`, and `git_worktree` based on agent intent and repository capability
- [x] 5.2 Add worktree/branch allocation for modifying agent runs, with explicit fallback to `shared_workspace` when Git isolation is unavailable
- [x] 5.3 Persist workspace metadata, code-change summaries, verification summaries, and generic artifacts for each agent run
- [x] 5.4 Ensure synthesis consumes agent artifacts and workspace references when producing merge-oriented summaries and next-step guidance

## 6. Slice D — Frontend Information Architecture & UX

- [x] 6.1 Add the new `Orchestrations` primary route/page and keep the existing workflow route available as `Legacy Workflows` with compatibility redirects
- [x] 6.2 Build the top orchestration input area with goal input, workspace context, submit state, and optimistic creation/loading handling
- [x] 6.3 Build orchestration detail loading that renders rounds in order and places sibling agent cards in the same row
- [x] 6.4 Build the selection/detail panel for agent logs, artifacts, code-change summaries, synthesis output, and orchestration control actions
- [x] 6.5 Wire orchestration-specific WebSocket updates into the new pages while reusing existing execution log streaming and log-tail fetch behavior
- [x] 6.6 Update topbar/navigation copy so the product clearly distinguishes `Orchestrations`, `Chat`, and `Legacy Workflows`

## 7. Validation, Rollout, and Handoff

- [x] 7.1 Add store/service tests for orchestration creation, first-round materialization, round barrier behavior, synthesis decisions, continue flow, retry flow, and restart recovery
- [x] 7.2 Add API and WebSocket integration tests covering orchestration create/detail/cancel/continue and execution event correlation for agent runs
- [x] 7.3 Manually verify the 0→1 happy path (`goal input → master planning → round fan-out → parallel agent execution → synthesis → continue into next round`)
- [x] 7.4 Manually verify that `Legacy Workflows` still behaves as before, including workflow start, DAG rendering, execution logs, cancel, and retry
- [x] 7.5 After implementation lands, update `PROJECT_STRUCTURE.md` for new orchestration files and run `/opsx:archive` to merge delta specs into baseline specs
