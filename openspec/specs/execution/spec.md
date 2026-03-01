# Execution 生命周期

## Purpose

Execution 是 Node 的一次运行记录。一个 Node 可以有多个 Execution（retry、重跑）。Execution 管理进程启动、日志流式输出、退出处理全过程。

## Requirements

### Execution State Machine

The system MUST support execution states: `queued`, `running`, `succeeded`, `failed`, `canceled`, `timeout`. The system MUST enforce transitions: `queued → running`, `running → succeeded | failed | canceled | timeout`.

#### Scenario: Successful execution

- **WHEN** a queued execution is started by the scheduler
- **THEN** status transitions to `running`
- **AND** upon process exit with code 0, status transitions to `succeeded`

#### Scenario: Failed execution

- **WHEN** a running execution's process exits with non-zero code
- **THEN** status transitions to `failed`
- **AND** the exit code and error message are recorded

### Log Management

The system MUST create one log file per execution at `~/.local/share/vibe-tree/logs/{execution_id}.log`. The system MUST append log data in real-time using buffered I/O with periodic flush. The system MUST provide a log tail API at `GET /api/v1/executions/{id}/log?tail=2000` (bytes).

#### Scenario: Real-time log writing

- **WHEN** a running execution produces output
- **THEN** output is appended to the log file in real-time
- **AND** log chunks are pushed via WebSocket `node.log` events

#### Scenario: Log tail retrieval

- **WHEN** client requests GET /api/v1/executions/{id}/log?tail=2000
- **THEN** the last 2000 bytes of the log file are returned

### WebSocket Events

The system MUST push `execution.started` (with execution_id and node_id), `node.log` (with chunk as UTF-8 string including ANSI escapes), and `execution.exited` (with exit_code and status) events via WebSocket.

#### Scenario: Execution lifecycle events

- **WHEN** an execution starts, produces output, and exits
- **THEN** `execution.started` is pushed at start
- **AND** `node.log` events are pushed for each output chunk
- **AND** `execution.exited` is pushed upon completion

### Cancellation Mechanism

The system MUST support cancel via `POST /api/v1/executions/{id}/cancel`. For process mode, the system MUST send SIGTERM first, wait for `kill_grace_ms` (default 1500ms), then SIGKILL if the process has not exited. For SDK mode, the system MUST use context cancellation.

#### Scenario: Cancel a running execution

- **WHEN** user calls cancel API on a running execution
- **THEN** SIGTERM is sent to the process
- **AND** after 1500ms grace period, SIGKILL is sent if process still alive
- **AND** execution status is set to `canceled`

### Retry Support

The system MUST support retry via `POST /api/v1/nodes/{id}/retry`. Retry MUST create a new Execution record, preserving historical Execution records. The node's `last_execution_id` MUST be updated to point to the new Execution.

#### Scenario: Retry a failed node

- **WHEN** user calls retry API on a failed node
- **THEN** a new execution is created with incremented attempt number
- **AND** the node's last_execution_id points to the new execution
- **AND** the previous execution record is preserved
