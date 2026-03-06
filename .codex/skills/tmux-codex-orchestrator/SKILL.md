---
name: tmux-codex-orchestrator
description: 用一个主 Codex 编排多个 tmux 终端中的 Codex worker，支持“先出 ORCH_PLAN.md 审查再执行”与“自然语言直接执行”双路径，支持同题多解并产出第六合成分支，以及多任务并行后统一整合。支持 analyze 只读分析模式（不创建 worktree、不写代码）与 modify 执行模式。Use when users want one lead Codex to split tasks, run parallel Codex workers in tmux, interrupt or inject worker prompts during execution, and either synthesize code branches or aggregate analysis-only outputs while preserving worker branches and worktrees.
---

# Tmux Codex Orchestrator

## 目标

- 用主 Codex 自动完成：任务拆解 -> 计划审查 -> tmux 并行执行 -> 结果整合。
- 默认先产出 `ORCH_PLAN.md`；命中“直接执行”关键词时可跳过审查。
- 保留所有 worker 分支与 worktree；默认仅关闭 tmux session。

## 命令入口

```bash
bash .codex/skills/tmux-codex-orchestrator/scripts/tmux-orch.sh <command> [args...]
```

## Commands

- `doctor`
- `draft --goal "<需求>" [--mode auto|same-task|split-task] [--execution-kind auto|modify|analyze] [--workers N]`
- `revise --run <run_id> --feedback "<反馈>"`
- `run --run <run_id>`
- `control --run <run_id> --worker <wXX> --action stop|inject|restart [--prompt "<新指令>"]`
- `status --run <run_id> [--json]`
- `synthesize --run <run_id> [--branch orchestrator/<run>/synth]`
- `report --run <run_id>`
- `close --run <run_id>`

## 核心流程

1. 执行 `draft` 生成 `ORCH_PLAN.md`。
2. 若用户未给出“直接执行”意图，先让用户审查计划表。
3. 用户确认后执行 `run`，创建 tmux session 并启动 worker。
4. 运行中可用 `control` 对单个 worker 执行 stop/inject/restart。
5. 执行 `status` 追踪进度；所有 worker 完成后自动关闭 session。
6. 执行 `synthesize` 生成第六分支并输出综合报告。
7. 需要时执行 `report` 查看总结，`close` 手动关闭 session。

## 模式说明

- `same-task`：多个 worker 对同一目标做不同策略实现；主流程会选基线并合成到第六分支。
- `split-task`：多个 worker 处理不同子任务；主流程会在第六分支串行整合。
- `auto`：按目标文本自动判定模式。
- `execution-kind=modify`：执行修改流程，会创建 worker 分支和 worktree。
- `execution-kind=analyze`：仅分析，不创建 worker worktree，不执行代码写入。
- `execution-kind=auto`：命中“只分析/不修改/分析分支”等关键词时自动切到 `analyze`，否则为 `modify`。

## 直接执行规则

- 仅靠自然语言关键词触发跳过审查：`直接执行`、`不用表格`、`跳过表格`、`无需审查`、`直接开跑`、`马上执行`。
- 未命中关键词时，默认走“先表格审查”流程。

## 硬约束

- 不删除 worker 分支。
- 不删除 worker worktree。
- 合成结果写入 `orchestrator/<run>/synth`（可覆盖为自定义分支）。
- `analyze` 模式下不做分支合成，产出分析报告并聚合 worker 回复。
- 默认并发上限：
  - `split-task` 未指定 `--workers` 时最多 `8`。
  - `same-task` 未指定 `--workers` 时由主 Codex按目标复杂度估算。

## 产物位置

- 计划表：`ORCH_PLAN.md`
- 运行状态：`.codex/skills/tmux-codex-orchestrator/.state/<run_id>.json`
- worker 日志：`.codex/skills/tmux-codex-orchestrator/.logs/<run_id>/`
- worker 最终回复：`.codex/skills/tmux-codex-orchestrator/.results/<run_id>/`
- 综合报告：`.codex/skills/tmux-codex-orchestrator/.reports/<run_id>-synthesis.md`
