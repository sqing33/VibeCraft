## Context

当前 `POST /api/v1/settings/experts/generate` 直接接收 message 数组并返回一个 draft，前端仅在内存中维护对话记录。这导致专家生成无法恢复、无法追踪版本，也无法基于旧上下文继续优化某个专家。用户明确要求把生成过程做成长对话式工作流，并把历史与最终专家参数一起保留下来。

现有仓库已经有 chat session 的 SQLite 存储模式，可以复用“session + messages + derived records”的思路，但专家生成与普通聊天不同：它必须记录每轮 draft 快照，并能把某个快照发布为正式 expert 配置。

## Goals / Non-Goals

**Goals:**
- 提供持久化的 expert builder session / messages / snapshots。
- 支持继续基于某个专家或某个历史 session 进行长对话微调。
- 支持把快照发布成专家，并记录来源 session / snapshot。
- 保持生成器仍可使用 demo 或已配置模型列表。

**Non-Goals:**
- 本次不实现多人协作审批流。
- 本次不实现快照 diff 可视化编辑器，只做版本列表与预览。
- 本次不把完整历史写入 config.json，历史仍放在 SQLite 中，config 只保存来源引用。

## Decisions

### 1. 使用独立的 builder session 三表模型

新增三张表：
- `expert_builder_sessions`
- `expert_builder_messages`
- `expert_builder_snapshots`

会话保存元数据；消息保存对话历史；快照保存每轮生成后的 expert 草稿 JSON。

**备选方案：**把消息 JSON 直接塞进 config 或单表。缺点是检索与扩展都差，不利于后续快照版本管理。

### 2. 保持生成 API 为“追加消息并生成新快照”

新增 `POST /api/v1/settings/experts/sessions/:id/messages`：
- 写入用户消息
- 调 builder 生成 assistant reply + draft
- 写入 assistant 消息
- 生成新 snapshot
- 返回最新 session 详情

这样前端只需围绕 session id 进行连续对话。

### 3. 发布时把 session / snapshot 引用写入 expert 配置

在 `ExpertConfig` 上新增：
- `builder_session_id`
- `builder_snapshot_id`

这样 expert 本身不保存整段历史，但能追溯回来源会话，用于“继续优化”。

### 4. ExpertSettingsTab 升级为 session 工作台

AI 创建专家 modal 升级为：
- 会话选择 / 新建会话
- 左侧对话历史
- 右侧当前 draft 与快照列表
- 支持对已有 expert 点击“继续优化”，自动定位或创建关联 session

### 5. Snapshot 保存完整 draft JSON，而不是只存增量

每轮快照保存完整 `draft_json`，避免回放时依赖历史 patch 叠加。

**备选方案：**保存 diff。优点是节省空间；缺点是实现复杂，当前收益不高。

## Risks / Trade-offs

- [生成历史会增长较快] → 通过会话状态归档与快照列表分页控制规模。
- [发布后的 expert 与后续会话分叉] → 通过 `builder_session_id` / `builder_snapshot_id` 保留来源回溯。
- [UI 复杂度上升] → 使用 session 下拉 + current snapshot 预览 + 历史快照简表，避免一次塞太多控件。

## Migration Plan

1. 扩展 SQLite schema 到新版本，创建 session/message/snapshot 表。
2. 新增 store 方法与 API。
3. 升级前端专家工作台为 session 驱动。
4. 增加测试覆盖 create session / append message / create snapshot / publish。
5. 更新 spec 与结构索引，归档变更。

## Open Questions

- 是否需要支持“同一 session 切换 builder model”。本次先固定 builder model 到 session 级别。
- 是否需要在快照层记录自动评测结果。可作为后续增强。
