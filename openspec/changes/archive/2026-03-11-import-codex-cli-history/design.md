## Context

Codex CLI 会把历史线程写在 `~/.codex/state_*.sqlite` 的 `threads` 表里，具体 turn 过程则保存在 `threads.rollout_path` 指向的 JSONL 文件中。`vibe-tree` 已有完整的 chat transcript 与 turn timeline 数据模型，前端也已经能渲染 `tool`、`thinking`、`progress`、`answer` 等条目，因此这次变更的关键不是新建一套历史展示模型，而是把 Codex 历史稳定映射进现有模型。

## Goals / Non-Goals

**Goals:**
- 只从 `~/.codex` 枚举和导入历史。
- 在列表中展示可读标题，避免用户看到 `Codex <uuid>` 或整段 worker prompt。
- 导入后的 session 继续复用现有 chat 页面，不新建独立历史页面。
- 导入时保留足够的过程信息，使现有 timeline UI 能直接还原工具调用与阶段性进度。
- 导入动作幂等，同一个 Codex thread 不会重复创建多个本地 session。

**Non-Goals:**
- 不导入 `~/.local/share/vibe-tree/managed-clis/codex` 的 managed runtime 历史。
- 不尝试解密 Codex rollout 中的 `encrypted_content` reasoning。
- 不重做现有聊天消息 UI 或 timeline 展示组件。
- 不在本次变更中实现历史自动同步或后台定时导入。

## Decisions

### 1. 使用 `threads.title` 的解析结果作为导入后标题

- 导入标题优先级固定为：
  1. 解析后的 `threads.title`
  2. `threads.first_user_message`
  3. rollout JSONL 中第一条真实 `user_message`
  4. `Codex <short-thread-id>`
- 其中 `threads.title` 需要做结构化压缩：
  - 普通用户问题直接取首行并截断。
  - `Recent conversation:` 标题优先提取 `Current user input:` 或首个 `USER:`。
  - worker / orchestrator prompt 优先提取 `task_title`，否则退化为 `并行 worker <id>`。
- 原因：用户选中导入前首先依赖可读标题，标题解析质量比导入后补说明更重要。

### 2. 后端直接把历史回填到现有 chat schema

- 不新增“外部历史表”或“只读虚拟 session”。
- 导入后直接写入现有 `chat_sessions`、`chat_messages`、`chat_turns`、`chat_turn_items`。
- 原因：前端现有聊天页、turn feed、fork/resume 逻辑都围绕这几张表构建，直接回填复用成本最低。

### 3. 导入使用显式 turn 写入，而不是复用在线 append 语义

- 现有在线写入路径会让每条 message 自增 `last_turn`，这适合流式运行，但不适合离线导入历史 transcript。
- 导入需要让同一轮 user / assistant 共享同一个 `turn` 值，并让 tool / reasoning / progress 条目挂到对应 `chat_turn` 下。
- 因此 store 新增 import helper，按显式 turn 批量写入消息与 timeline。

### 4. 前端入口放在聊天页左侧会话列表头部

- 在 `#/chat` 左栏“会话”标题区域新增 `导入 Codex 历史` 按钮。
- 点击后弹出 Modal，内部完成线程加载、搜索、选择与导入。
- 原因：用户明确要求在前端提供入口，而且导入结果本质上就是聊天会话，最自然的位置就是会话列表附近。

## Data Flow

1. 前端打开导入弹窗，请求 `GET /api/v1/codex-history/threads`。
2. 后端读取 `~/.codex/state_*.sqlite` 最新库，枚举 threads，解析 `display_title`，并标记是否已导入。
3. 用户选择若干 thread 后，前端调用 `POST /api/v1/codex-history/import`。
4. 后端按 thread id 读取 thread 元数据与 rollout JSONL，组装显式 transcript 与 timeline。
5. store 事务性写入 chat session / messages / turns / turn items。
6. 前端刷新会话列表并优先切到首个新导入 session。

## Risks / Trade-offs

- [Codex 源 SQLite 可能部分损坏] → 读取时使用 `mode=ro&immutable=1`，列表查询控制为只读扫描；导入单条 thread 使用按 id 精确查询，降低受损页影响范围。
- [rollout 事件结构随 Codex 版本变化] → 采用“宽松解析 + 保底映射”：只依赖稳定字段（`type`、`call_id`、`message`、`last_agent_message`），无法识别的事件直接忽略。
- [tool output 格式并不统一] → 统一落到 `tool` 条目，并尽量填充 `stdout` / `stderr` / `exit_code`；若无法拆解则保留原始输出文本。
- [已导入 session 可能来自旧版差标题数据] → 本次以幂等跳过为主，不主动重写已有 session 内容，避免覆盖用户后续手工修改。
