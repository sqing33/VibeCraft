## 1. Spec and backend service

- [x] 1.1 新增 `codex-history-import` delta spec，并补充 UI 变更说明
- [x] 1.2 实现 `backend/internal/codexhistory`：扫描 `~/.codex/state_*.sqlite`、解析标题、读取 rollout JSONL
- [x] 1.3 新增显式 turn 导入 store helper，并保证按 `cli_tool_id=codex + cli_session_id=thread_id` 幂等

## 2. API and frontend entry

- [x] 2.1 新增 `GET /api/v1/codex-history/threads` 与 `POST /api/v1/codex-history/import`
- [x] 2.2 在聊天页左侧会话头部增加“导入 Codex 历史”入口
- [x] 2.3 实现导入弹窗：加载列表、搜索过滤、条目选择、导入后刷新会话并切换到新导入会话

## 3. Verification and docs

- [x] 3.1 为标题解析、rollout 导入与 API handler 增加测试
- [x] 3.2 更新 `PROJECT_STRUCTURE.md` 中新增模块与入口说明
- [x] 3.3 运行 `go test ./...`、`npm run build` 与 `openspec validate --all`
