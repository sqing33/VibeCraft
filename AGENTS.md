# vibe-tree AGENTS 使用说明

本文件只负责定义「该用哪个 skill」与「如何协作」，具体规范细节以各 skill 的 `SKILL.md` 为准。

## 1. 必须遵循的基础流程

1. 改动前先读 `PROJECT_STRUCTURE.md`，先定位再实现。
2. **新功能/重大变更（涉及新增 API、新模块、跨文件改动、行为变更）必须先执行 `/opsx:propose`，生成 proposal 并对齐需求。** 若用户未主动提及，AI 应主动提醒并询问是否需要先 propose。
3. 在目标目录内做定向检索（优先 `rg`），避免全仓盲搜。
4. 根据任务类型选择并执行对应 skill（见下节）。
5. 若新增关键文件/职责变化，必须同步更新 `PROJECT_STRUCTURE.md`。
6. **完成变更后执行 `/opsx:archive`，将 delta specs 合并到基线 specs。**
7. **自动提醒规则**：当用户的请求明显属于新功能或重大变更时，AI 必须在开始编码前提醒用户："这个变更建议先通过 `/opsx:propose` 生成规范，是否现在执行？"

## 2. 技能路由（按需启用）

| skill                 | 何时使用                                             | 文件路径                                                     |
| --------------------- | ---------------------------------------------------- | ------------------------------------------------------------ |
| `opsx:propose`        | 新功能/重大变更前，生成 proposal + specs + design + tasks | `.claude/skills/openspec-propose/SKILL.md`                   |
| `opsx:apply`          | 按 OpenSpec change 中的 tasks 实施开发                   | `.claude/skills/openspec-apply-change/SKILL.md`              |
| `opsx:archive`        | 完成变更后归档，将 delta specs 合并到基线                | `.claude/skills/openspec-archive-change/SKILL.md`            |
| `opsx:explore`        | 探索想法、调查问题、厘清需求（编码前思考）               | `.claude/skills/openspec-explore/SKILL.md`                   |
| `vibe-tree-standards` | 需要统一日志、注释、提交命名；或做功能定位时             | `.codex/skills/vibe-tree-standards/SKILL.md`                 |
| `worktree-lite`       | 需要轻量 worktree 开发流（创建、审查、询问是否合并） | `.codex/skills/worktree-lite/SKILL.md`                       |
| `ui-ux-pro-max`       | UI/UX 设计与实现（规划/设计/优化界面与交互）         | `/home/sqing/.cc-switch/skills/ui-ux-pro-max/SKILL.md`       |
| `skill-creator`       | 需要新增或更新 skill                                 | `/home/sqing/.codex/skills/.system/skill-creator/SKILL.md`   |
| `skill-installer`     | 需要列出/安装 skill（curated 或 GitHub）             | `/home/sqing/.codex/skills/.system/skill-installer/SKILL.md` |

## 3. 选择与执行规则

1. 用户点名某个 skill（如 `$vibe-tree-standards`）时，必须使用该 skill。
2. 任务明显匹配 skill 描述时，必须自动启用对应 skill。
3. 多个 skill 同时匹配时，使用「最小覆盖集合」，并声明执行顺序。
4. 若 skill 文件缺失或不可读，需说明问题并采用最接近的回退方案继续执行。

## 4. 交付最小要求

1. 说明本次使用了哪些 skill，以及为什么。
2. 列出改动文件与目的。
3. 列出验证命令（或说明未执行原因）。
