## 1. Runtime Foundation

- [x] 1.1 扩展 expert/config 结构，显式支持 `runtime_kind=cli|process|sdk_helper`、`cli_family`、`helper_only` 等元数据
- [x] 1.2 引入 CLI adapter / wrapper contract，定义 `summary.json`、`artifacts.json`、可选 `session.json` / `patch.diff` 等标准输出
- [x] 1.3 更新 `runner.MultiRunner` 与 daemon 依赖注入，新增 CLI 主运行器并把 `SDKRunner` 限定到 helper-only 场景
- [x] 1.4 增加 expert resolve / runner routing 单元测试，覆盖 CLI、process、sdk_helper 三类分流

## 2. Workflow & Orchestration Migration

- [x] 2.1 更新 workflow start / scheduler 路径，使 master 与 AI worker 默认解析并启动 CLI runtime
- [x] 2.2 更新 orchestration manager，使 master planning、agent run、synthesis 默认解析并启动 CLI runtime
- [x] 2.3 为 workflow/orchestration 持久化并返回 runtime 元数据、artifact 目录引用、CLI session 引用（如适用）
- [x] 2.4 增加 workflow/orchestration 集成测试，验证日志流、取消、重试、继续控制仍可复用共享 execution surface

## 3. Chat Migration

- [x] 3.1 将 chat turn 主执行路径从 SDK provider 调用切换到 CLI runtime，同时保留现有 `chat.turn.*` 流式事件形状
- [x] 3.2 移除新 turn 对 provider anchor 的依赖，改为使用 session summary、recent history 与 runtime session metadata 维持连续性
- [x] 3.3 更新 chat store/schema 与 API payload，持久化 CLI runtime 相关字段并兼容历史 session/message 读取
- [x] 3.4 保持附件、fork、manual compact、automatic compaction 在 CLI chat 下继续可用，并补齐集成测试

## 4. SDK Helper Isolation

- [x] 4.1 将 thinking translation、`/api/v1/settings/llm/test`、经批准的单次辅助生成能力显式标记为 `sdk_helper`
- [x] 4.2 确保 chat/workflow/orchestration 的默认 expert 选择不会落到 helper-only SDK expert
- [x] 4.3 如保留 helper SDK fallback，限制其仅作用于 helper lane，并增加对应测试

## 5. Compatibility & Cleanup

- [x] 5.1 处理与 `chat-per-message-model-routing` change 的重叠：明确合并、取代或废弃策略
- [x] 5.2 保留旧 provider anchor / SDK 相关数据结构的向后兼容读取，但禁止新主链路继续写入这类 runtime 假设
- [x] 5.3 更新必要的 API / WS 类型与后端契约文档，确保前端可在不重做主要交互的前提下消费新字段

## 6. Manual Verification

- [x] 6.1 验证 workflow 启动后 master/worker 均通过 CLI runtime 执行，并保留 execution log / cancel / retry 能力
- [x] 6.2 验证 orchestration 的 planning、agent run、synthesis 默认走 CLI runtime，详情页可查询到 runtime/artifact 元数据
- [x] 6.3 验证 `#/chat` 在 CLI turn 路径下仍支持流式回复、附件、fork、manual compact 与 daemon 重启后继续会话
- [x] 6.4 验证 thinking translation 与 `POST /api/v1/settings/llm/test` 仍能通过 SDK helper 正常工作
