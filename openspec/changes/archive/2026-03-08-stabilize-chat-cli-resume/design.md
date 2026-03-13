## Context

当前 `main` 已具备：
- `chat_sessions` 上的 `cli_tool_id/model_id/cli_session_id`
- `runCLITurn()` 的 resume-first 尝试
- wrapper 级 `VIBECRAFT_RESUME_SESSION_ID`

但稳定性还不足：
- UI `Select` 只有 `selectedKeys`，没有显式 render label，工具切换/默认值变化后容易显示空白。
- CLI session 提取依赖 wrapper 的日志/JSON 解析，解析不稳时会导致 session id 未落库，后续 turn 就回退到本地重建 prompt。
- 旧数据库若没经历完整迁移，查询 `cli_tool_id` 会报错；需要继续保证迁移逻辑对旧库安全。

## Goals / Non-Goals

**Goals:**
- 稳定模型下拉显示
- 稳定 resume-first 对话链路
- 保持旧库兼容性

**Non-Goals:**
- 不再引入新的主抽象
- 不改 workflow/orchestration 的行为
- 不重做 settings 页面结构

## Decisions

### 1. 模型 `Select` 使用显式 renderValue

无论 `selectedKeys`、session 默认值还是工具切换后的默认模型，都通过当前工具模型池查找并渲染 label，避免显示空白。

### 2. wrapper 负责输出 `session.json`，manager 负责持久化

wrapper 从 Codex/Claude 输出中提取 `thread_id/session_id`，统一写到 `session.json`。
`runCLITurn()` 只读取 artifact，不自己解析 stdout 结构。

### 3. resume 失败时仅一次回退

`runCLITurn()` 先尝试 resume；失败时回退到本地重建 prompt，并允许 wrapper 写出新的 session id 覆盖旧值。

### 4. 迁移继续采用“老库建表带新列 + v8 补列”双保险

保证：
- 新库直接建好新列
- 老库通过 `migrateV8` 补列

## Risks / Trade-offs

- wrapper 仍受 CLI 输出格式影响 → 通过 `session.json` 做单一契约出口减轻耦合
- resume 失败时仍可能回退到本地重建 → 这是故意保留的兜底，不视为失败
