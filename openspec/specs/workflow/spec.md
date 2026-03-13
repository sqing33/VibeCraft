# Workflow 管理

## Purpose

Workflow 是用户创建的一次"要完成的任务"，对应 Kanban 卡片。每个 Workflow 包含一个 master 节点（规划/拆解）和零或多个 worker 节点（执行）。支持两种运行模式（auto/manual）并可运行中动态切换。

## Requirements

### Workflow 状态机

The system MUST support the following workflow states: `todo`, `planning`, `pending_approval`, `running`, `done`, `failed`, `canceled`. The system MUST enforce valid state transitions: `todo → planning`, `planning → pending_approval | running | failed | canceled`, `pending_approval → running`, `running → done | failed | canceled`.

#### Scenario: Workflow transitions from todo to planning

- **WHEN** user clicks Start on a todo workflow
- **THEN** workflow status changes to `planning`
- **AND** a master node with execution is created

#### Scenario: Planning completes in manual mode

- **WHEN** master node outputs a valid DAG in manual mode
- **THEN** workflow status changes to `pending_approval`
- **AND** all worker nodes are set to `pending_approval`

#### Scenario: Planning completes in auto mode

- **WHEN** master node outputs a valid DAG in auto mode
- **THEN** workflow status changes to `running`
- **AND** scheduler begins executing worker nodes

### Workflow CRUD API

The system MUST provide REST APIs for workflow management: `POST /api/v1/workflows` to create, `GET /api/v1/workflows` to list, `GET /api/v1/workflows/{id}` to get details, `PATCH /api/v1/workflows/{id}` to update title/workspace/mode.

#### Scenario: Create a new workflow

- **WHEN** user sends POST /api/v1/workflows with title, workspace, mode, and master expert
- **THEN** a new workflow is created with status `todo`
- **AND** the workflow is returned with a generated `wf_` prefixed ID

### Workflow Run Control

The system MUST provide run control APIs: `POST /api/v1/workflows/{id}/start` to start, `POST /api/v1/workflows/{id}/approve` to approve pending nodes (manual mode), `POST /api/v1/workflows/{id}/cancel` to cancel.

#### Scenario: Cancel a running workflow

- **WHEN** user calls cancel API on a running workflow
- **THEN** all running executions are canceled (SIGTERM → SIGKILL)
- **AND** all queued/pending nodes are marked `canceled`
- **AND** workflow status changes to `canceled`

### Execution Breakpoint Toggle

The system MUST support dynamic mode switching between auto and manual during workflow execution. When switching auto → manual, the scheduler MUST stop launching new nodes and all queued nodes MUST transition to `pending_approval`. When switching manual → auto, all dependency-satisfied `pending_approval` nodes MUST transition to `queued`.

#### Scenario: Switch from auto to manual during execution

- **WHEN** user switches a running auto workflow to manual
- **THEN** already running nodes continue executing
- **AND** queued nodes transition to `pending_approval`
- **AND** future runnable nodes enter `pending_approval` instead of `queued`

#### Scenario: Switch from manual to auto

- **WHEN** user switches a manual workflow to auto
- **THEN** all dependency-satisfied `pending_approval` nodes transition to `queued`
- **AND** scheduler resumes automatic scheduling

### Workflow Data Model

The system MUST store workflow fields: id, title, workspace_path, mode (auto|manual), status, created_at, updated_at, error_message, summary. The system MUST broadcast `workflow.updated` WebSocket events on any workflow state change.

#### Scenario: Workflow state change broadcasts event

- **WHEN** workflow status changes from `running` to `done`
- **THEN** a `workflow.updated` event is broadcast via WebSocket
- **AND** the event contains the updated workflow data

### Requirement: Workflow planning and AI worker runs MUST use CLI runtime by default
When a workflow run requires AI planning or AI worker execution, the system MUST resolve a CLI-capable expert by default and start that work through the shared CLI runtime. Existing workflow lifecycle APIs, state transitions, execution logging, and cancellation surfaces MUST remain compatible.

#### Scenario: Workflow master planning starts through CLI runtime
- **WHEN** a user starts a workflow that uses an AI master expert
- **THEN** the master execution is launched through the CLI runtime by default
- **AND** the workflow still enters the existing planning lifecycle

#### Scenario: Workflow AI worker runs through CLI runtime
- **WHEN** the scheduler starts an AI worker node in a running workflow
- **THEN** the resolved worker execution uses the CLI runtime by default
- **AND** the resulting execution remains observable through the existing execution log and cancel APIs

### Requirement: Workflow primary CLI experts MUST use tool-bound default models
When workflow planning or AI worker execution resolves to a primary CLI tool expert, the execution MUST use that tool's configured default model unless explicitly overridden.

#### Scenario: Workflow master uses codex default model
- **WHEN** workflow planning runs through the codex tool expert
- **THEN** the execution uses the default model configured for the `codex` CLI tool

### Requirement: Workflow-facing path defaults MUST use the current runtime prefix
Workflow-facing documentation, diagnostics, and default execution path references MUST use the current runtime prefix instead of legacy project naming.

#### Scenario: User checks workflow-related runtime paths
- **WHEN** the user inspects workflow diagnostics, logs, or documentation after the rename
- **THEN** the referenced default workflow log and data paths use the current runtime prefix
