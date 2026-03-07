# ORCH_PLAN

- run_id: `stream-all-20260308-034813`
- goal: 直接执行：只读分析 .github-feature-analyzer 目录下的全部项目（BloopAI-vibe-kanban、echoVic-blade-agent-runtime、fengshao1227-ccg-workflow、Haleclipse-codex、iOfficeAI-AionUi、liu-ziting-ThinkFlowAI、octocat-Hello-World、paopaoandlingyia-PrismCat、parallized-maple、Soein-swarmesh、ZekerTop-ai-cli-complete-notify）。

统一分析问题：
1. 每个项目是否存在“CLI 输出实时传到前端/UI/终端”的实现；
2. 如果有，CLI 是如何被启动的（直接进程 / PTY / SDK / SSE / WebSocket / app server 等）；
3. 实际消费的输出粒度是什么（逐 token / 逐字符 / 逐段 delta / item 完成态 / 文件日志 tail / 轮询）；
4. 为什么用户感知会更流畅；
5. 与当前仓库 vibe-tree 的 Codex CLI 聊天实现相比，关键差异是什么；
6. 如果项目与该问题无关，也要明确说明“无相关实现或证据不足”。

要求：
- 每个 worker 负责一个项目；
- 只读分析，不修改任何文件；
- 输出必须包含关键文件路径和行号证据；
- 最终给出一个跨项目汇总，按“最接近我们问题的实现”排序。
- mode: `split-task`
- execution_kind: `analyze`
- execution_policy: `direct`
- base_branch: `main`
- created_at: `2026-03-07T19:48:29+00:00`

| run_id | mode | worker_id | task_title | task_scope | strategy | base_branch | worker_branch | worktree_path | verify_cmd | status | session_id | result_ref | notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| stream-all-20260308-034813 | split-task | w01 | 每个项目是否存在“CLI 输出实时传到前端/UI/终端”的实现； | 每个项目是否存在“CLI 输出实时传到前端/UI/终端”的实现； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w01.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w02 | 如果有，CLI 是如何被启动的（直接进程 / PTY / SDK / SSE / WebSocket / app server 等）； | 如果有，CLI 是如何被启动的（直接进程 / PTY / SDK / SSE / WebSocket / app server 等）； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w02.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w03 | 实际消费的输出粒度是什么（逐 token / 逐字符 / 逐段 delta / item 完成态 / 文件日志 tail / 轮询）； | 实际消费的输出粒度是什么（逐 token / 逐字符 / 逐段 delta / item 完成态 / 文件日志 tail / 轮询）； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w03.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w04 | 为什么用户感知会更流畅； | 为什么用户感知会更流畅； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w04.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w05 | 与当前仓库 vibe-tree 的 Codex CLI 聊天实现相比，关键差异是什么； | 与当前仓库 vibe-tree 的 Codex CLI 聊天实现相比，关键差异是什么； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w05.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w06 | 如果项目与该问题无关，也要明确说明“无相关实现或证据不足”。 | 如果项目与该问题无关，也要明确说明“无相关实现或证据不足”。 | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w06.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w07 | 每个 worker 负责一个项目； | 每个 worker 负责一个项目； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w07.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w08 | 只读分析，不修改任何文件； | 只读分析，不修改任何文件； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w08.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w09 | 输出必须包含关键文件路径和行号证据； | 输出必须包含关键文件路径和行号证据； | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w09.md | subtasks=1 |
| stream-all-20260308-034813 | split-task | w10 | 最终给出一个跨项目汇总，按“最接近我们问题的实现”排序。 | 最终给出一个跨项目汇总，按“最接近我们问题的实现”排序。 | - | main | - | . | - | planned | last | .codex/skills/tmux-codex-orchestrator/.results/stream-all-20260308-034813/w10.md | subtasks=1 |

## Notes

- 默认先审查表格；命中直接执行关键词时可直接 `run`。
- execution_kind=analyze 时：不创建 worktree，不进行代码写入，worker 以只读沙箱执行。
- 修改计划后可执行：`tmux-orch.sh revise --run <run_id> --feedback "..."`。
- 查看状态：`tmux-orch.sh status --run stream-all-20260308-034813`。
- 查看详情：`tmux-orch.sh inspect --run stream-all-20260308-034813`。
