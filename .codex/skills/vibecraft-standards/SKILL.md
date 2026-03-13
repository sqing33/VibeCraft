---
name: vibecraft-standards
description: "统一 VibeCraft 全仓库开发规范：运行日志格式、中文四要素注释、Git 提交标题 `type(scope): 中文信息`，并强制先读取 `PROJECT_STRUCTURE.md` 做功能定位。Use when users ask to standardize logs/comments/commit naming, request commit title suggestions, or ask where a feature is implemented."
---

# Vibe Tree Standards

## 目标

- 统一 `backend/`、`ui/`、`scripts/` 的日志、注释和提交命名风格，降低跨模块协作成本。
- 强制先看结构索引再改代码，减少“全仓库盲搜”导致的错误落点。
- 输出可直接用于 `git commit -m` 的提交标题文案，但不执行 commit。

## 触发范围

满足任一条件即启用本规范：

1. 用户要求统一或检查日志输出格式。
2. 用户要求统一或检查注释格式。
3. 用户要求生成/优化提交标题。
4. 用户询问“某功能在哪个文件”或需要按功能定位代码。
5. 对话中出现 `backend/`、`ui/`、`scripts/` 路径并涉及实现改动。

## 强制工作流（先结构，后定位）

1. 先打开 `PROJECT_STRUCTURE.md`，用功能关键词定位模块与候选文件。
2. 再在候选目录内做精确检索（优先 `rg`），避免全仓库盲搜。
3. 修改代码时同步应用本 Skill 的日志与注释规范。
4. 交付前生成提交标题并检查是否符合 `type(scope): 中文信息`。
5. 若新增核心文件、目录职责变化或重命名，必须同步更新 `PROJECT_STRUCTURE.md` 的目录职责与功能定位索引。

## 运行日志规范

### Backend（Go）

- 统一格式：
  `level=<INFO|WARN|ERROR|DEBUG> module=<模块> action=<动作> msg="<中文说明>" key=value...`
- 必填字段：`module`、`action`、`msg`。
- 涉及工作流执行链时必须追加：
  - `workflow_id=<wf_...>`
  - `node_id=<nd_...>`（如适用）
  - `execution_id=<ex_...>`（如适用）
- 级别使用建议：
  - `INFO`：流程关键里程碑（开始/结束/状态切换）
  - `WARN`：可恢复异常或回退逻辑
  - `ERROR`：流程失败、中断、不可恢复异常
  - `DEBUG`：诊断信息（默认可关闭）
- 示例：
  `level=INFO module=workflow-scheduler action=start-node msg="启动节点执行" workflow_id=wf_123 node_id=nd_456 execution_id=ex_789`

### UI（TS/React）

- 开发日志统一前缀：
  `[ui:<module>] <中文说明> key=value...`
- 建议使用：
  - `console.info("[ui:health] 健康检查成功 url=http://127.0.0.1:7777")`
  - `console.warn("[ui:health] 健康检查失败 reason=timeout")`
- 涉及流程执行时，附加同名 ID 字段：`workflow_id`、`node_id`、`execution_id`。

### 安全约束

- 禁止输出密码、完整 token、完整 cookie、隐私原文。
- 如需排查，仅输出脱敏值（例如后四位或 hash）。

## 注释规范（中文四要素）

### 必写范围

- Go：
  - 所有导出函数（大写开头）
  - 所有 HTTP Handler
  - 核心 Service/Runner/Storage 流程函数
  - 关键并发入口（goroutine 启动点、调度入口）
- TS/React：
  - 导出函数/Hook
  - 复杂状态流转逻辑
  - 关键副作用逻辑（例如 WS 路由、日志流拼接、订阅释放）

### 注释模板（四要素）

必须覆盖以下四项（可在同一段注释中表达）：

1. 功能：该函数负责什么。
2. 参数/返回：关键输入和输出语义。
3. 失败场景：何时失败、如何表现。
4. 副作用：是否写库/写文件/发请求/启动异步任务。

Go 示例：

```go
// StartExecution 功能：启动单次执行并接入日志流。
// 参数/返回：接收执行规格 spec，返回 executionID 与错误信息。
// 失败场景：配置缺失、子进程启动失败或 PTY 初始化失败时返回 error。
// 副作用：创建子进程、写入日志文件、推送运行事件。
func StartExecution(spec RunSpec) (string, error) {
    // ...
}
```

TS 示例：

```ts
/**
 * 功能：订阅 execution 日志并将增量路由到对应终端。
 * 参数/返回：接收 ws 连接与 executionId，返回取消订阅函数。
 * 失败场景：ws 断开或 payload 非法时停止分发并记录告警。
 * 副作用：更新本地状态容器并注册事件监听器。
 */
export function subscribeExecutionLog(/* ... */) {}
```

### 禁止项

- 禁止复述代码字面行为（无信息增量）。
- 禁止“将来可能”类空泛描述。
- 禁止术语中英混乱导致语义不一致。

## Git 提交命名规范

- 固定格式：`type(scope): 中文信息`
- `scope` 必填，不允许省略。
- `type` 仅允许以下值：
  - `feat`
  - `fix`
  - `refactor`
  - `perf`
  - `docs`
  - `test`
  - `chore`
  - `build`
  - `ci`
  - `revert`

### 默认 scope 映射

- 改动 `backend/**` -> `backend`
- 改动 `ui/**` -> `ui`
- 改动 `desktop/**` -> `desktop`
- 改动 `scripts/**` -> `scripts`
- 改动 `openspec/**` -> `docs`
- 同时改动多个核心模块 -> `core`

### 输出约定

按以下顺序输出文案，不执行 commit：

1. `推荐：type(scope): 中文信息`
2. `备选1：type(scope): 中文信息`
3. `备选2：type(scope): 中文信息`
4. `理由：一句话解释 type/scope 与改动范围的匹配关系`

## 交付自检清单

1. 已先读取 `PROJECT_STRUCTURE.md` 并完成功能定位。
2. 已在目标模块内完成定向检索，未进行无差别盲搜实现。
3. 新增/修改日志符合统一格式，且包含 `module`、`action`、`msg`。
4. 涉及执行链路时已携带 `workflow_id`、`node_id`、`execution_id`（按适用性）。
5. 导出函数与复杂逻辑已补充中文四要素注释。
6. 注释中不存在无效复述或含糊描述。
7. 提交标题满足 `type(scope): 中文信息`，且 `scope` 已填写。
8. 提交标题输出包含推荐与备选项，并给出理由。
9. 若目录职责或关键文件发生变化，已同步更新 `PROJECT_STRUCTURE.md`。
