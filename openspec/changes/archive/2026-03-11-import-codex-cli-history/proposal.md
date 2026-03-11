## Why

当前聊天页只能展示由 `vibe-tree` 自己创建的 chat session，用户已经积累在 `~/.codex` 下的 Codex CLI 历史会话无法直接导入，也无法在前端挑选需要迁入的记录。即使读取到了 thread id，很多标题仍然是 `Codex <uuid>` 或长 worker prompt，缺少可读性，用户很难判断应该导入哪一条。

## What Changes

- 在 daemon 内新增 Codex 历史导入服务，扫描 `~/.codex/state_*.sqlite`，列出可导入 thread，并解析出可读 `display_title`。
- 新增导入 API，把选中的 Codex thread 回填到现有 chat session / message / turn timeline 表中，并保留 CLI thread id 以便后续 resume。
- 在聊天页左侧会话列表顶部新增“导入 Codex 历史”入口，弹窗展示所有可读标题，允许搜索、选择和批量导入。
- 导入时除了对话文本，还把 rollout JSONL 中的 tool call / tool output / reasoning / progress 等过程信息映射到现有 turn timeline。

## Capabilities

### New Capabilities
- `codex-history-import`: 从 `~/.codex` 枚举、解析并导入 Codex CLI 历史会话到本地聊天系统。

### Modified Capabilities
- `ui`: 聊天页增加 Codex 历史导入入口与选择弹窗。

## Impact

- 后端新增 Codex 历史读取、标题解析、JSONL 回填和 API handler。
- store 需要新增显式 turn 导入能力，确保 imported transcript 与 timeline 可恢复。
- 聊天页与 daemon client 需要增加导入弹窗、列表请求和导入动作。
