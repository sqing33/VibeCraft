# 调度器

## Purpose

按依赖关系和并发上限调度 DAG 中的 worker 节点。调度器是后端的核心组件，负责决定何时启动节点、处理失败传播、以及重启恢复。

## Requirements

### Scheduling Loop

The system MUST run a scheduling tick every 200ms. Each tick MUST: count currently running worker nodes, calculate available concurrency slots (`max_concurrency - running_count`), query runnable queued worker nodes via `ListRunnableQueuedWorkerNodes`, and for each runnable node: resolve expert, start execution, and watch for completion.

#### Scenario: Normal scheduling tick

- **WHEN** scheduler tick fires with 2 running nodes and max_concurrency=6
- **THEN** 4 available slots are calculated
- **AND** up to 4 queued runnable nodes are started

### Dependency Resolution

The system MUST require all incoming edge source nodes to be in `succeeded` status before a node can be scheduled. Nodes with no incoming edges MUST be immediately schedulable after workflow start.

#### Scenario: Node with satisfied dependencies

- **WHEN** node C depends on A and B (edges A→C, B→C)
- **AND** both A and B have status `succeeded`
- **THEN** node C becomes runnable and enters the scheduling queue

#### Scenario: Node with unsatisfied dependencies

- **WHEN** node C depends on A and B
- **AND** A is `succeeded` but B is still `running`
- **THEN** node C remains in `queued` and is not scheduled

### Concurrency Control

The system MUST enforce `execution.max_concurrency` (default 6). The system MUST NOT start new nodes when available slots are zero.

#### Scenario: Concurrency limit reached

- **WHEN** 6 nodes are running and max_concurrency=6
- **AND** another node becomes runnable
- **THEN** the runnable node stays in `queued` until a running node completes

### Fail-fast Strategy

The system MUST mark the workflow as `failed` when any worker node enters `error` status. The system MUST mark all unstarted `queued` nodes as `skipped`. Already `running` nodes MUST be allowed to finish (MVP default: no forced cancellation).

#### Scenario: Fail-fast propagation

- **WHEN** node B fails in a DAG with A→B→C
- **THEN** workflow is marked `failed`
- **AND** node C is marked `skipped`
- **AND** any other running nodes continue to completion

### Restart Recovery

The system MUST scan the database on daemon startup. All `running` executions MUST be marked as `failed` with reason `daemon_restarted`. Corresponding nodes MUST have their status set to `error`, allowing manual retry.

#### Scenario: Daemon restart recovery

- **WHEN** daemon restarts with 2 running executions in the database
- **THEN** both executions are marked `failed` with reason "daemon_restarted"
- **AND** their corresponding nodes are set to `error`
- **AND** users can retry these nodes

### Mode Interaction

The system MUST respect workflow mode during scheduling. In `manual` mode, newly runnable nodes MUST enter `pending_approval` instead of `queued`. In `auto` mode, newly runnable nodes MUST enter `queued` directly.

#### Scenario: Scheduling in manual mode

- **WHEN** a node's dependencies are satisfied in manual mode
- **THEN** the node enters `pending_approval` instead of `queued`
- **AND** the node waits for user approval before execution
