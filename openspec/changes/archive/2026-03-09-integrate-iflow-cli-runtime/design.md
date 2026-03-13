## Context

当前仓库的 CLI runtime 已经具备三层基础设施：

1. `cli_tools`：声明工具 id、协议族、cli family、默认模型与命令路径。
2. `expert.ResolveWithOptions()`：把 chat / repo library / orchestration 请求解析成最终 `RunSpec`。
3. `scripts/agent-runtimes/*.sh`：把统一环境变量翻译成具体 CLI 命令，并落 `summary.json` / `artifacts.json` / `session.json` / `final_message.md`。

现状中 `codex` 与 `claude` 的接入已跑通，但它们都不依赖 `llm.sources` 中的 source-level `base_url/api_key`。`iFlow CLI` 若以 `openai-compatible` 方式运行，则必须拿到所选 `model_id` 对应 source 的连接信息，因此不能只复制现有两条 wrapper。

另外，聊天恢复链路已经具备通用 contract：chat manager 会优先把 `sess.CLISessionID` 注入 `VIBECRAFT_RESUME_SESSION_ID`，wrapper 若写出 `session.json`，后端就会自动回写 session defaults。iFlow 需要接上这套 contract，而不是另起一套会话系统。

## Goals / Non-Goals

**Goals:**
- 把 `iFlow CLI` 作为第三个 CLI tool 接入现有 Settings / Chat / Repo Library 流程。
- 复用 `protocol_family=openai` 的模型池与 source 配置，不新增独立的 LLM provider 类型。
- 让 iFlow wrapper 支持：模型切换、OpenAI-compatible 认证、native resume、artifact contract、best-effort streaming。
- 从根因上补齐“CLI runtime 不继承模型 source 连接信息”的缺口，避免未来第三方 CLI 重复踩坑。
- 尽量保持对 Codex / Claude 的行为零回归。

**Non-Goals:**
- 不改变 orchestration / workflow 的默认 agent 选型优先级；`iflow` 先作为显式可选项接入。
- 不为 iFlow 增加专用的新配置页；仍复用现有 CLI tool settings。
- 不要求 iFlow 提供和 Codex app-server 同等级的结构化 runtime feed；第一阶段仅提供 line-based 增量与 artifact 真相源。
- 不把敏感认证信息写入仓库内配置文件。

## Decisions

### 1. 复用 `protocol_family=openai`，新增 `cli_family=iflow`
- **方案**：在 `cli_tools` 中新增 `iflow`，协议族仍为 `openai`，CLI family 为 `iflow`。
- **原因**：UI 模型过滤、默认模型选择、兼容性校验都已经围绕 `protocol_family` 工作，沿用现有语义成本最低。
- **备选**：新增 `provider=iflow` / `protocol_family=iflow`。
- **不选原因**：会连带修改 LLM settings、provider 校验、模型池过滤、API 兼容测试，收益低于成本。

### 2. 在 expert 解析层补齐 CLI 模型 source 运行时注入
- **方案**：当 `provider=cli` 且 `model_id` 解析到具体 `llm.models` / `llm.sources` 时，把 source 的 `api_key/base_url` 注入 `RunSpec.Env`，并同步写入 `VIBECRAFT_BASE_URL`。
- **原因**：这能从根因上修复所有“需要 OpenAI-compatible / Anthropic-compatible 连接信息的外部 CLI”场景，而不是只在 chat API 针对 iFlow 做临时 patch。
- **备选**：只在 `backend/internal/api/chat.go` 给 iFlow 单独补 env。
- **不选原因**：无法覆盖 repo library、workflow、orchestration 等其他 CLI 调用点。

### 3. iFlow wrapper 使用官方 non-interactive flags
- **方案**：wrapper 采用 `iflow -p <prompt> -m <model> -o <output-file> --yolo`，若已有 session id 则附加 `--resume <id>`。
- **原因**：这些参数已在官方 `--help` 中明确公开，兼容性最稳。
- **备选**：用位置参数 query 或 stdin 管道。
- **不选原因**：`-p/--prompt` 语义更明确；官方文档对 `--prompt-interactive` 明确限制 stdin；stdin 拼接规则不如 `-p` 清晰。

### 4. 用 `--output-file` 抓取 session-id，用 stdout 作为回答正文
- **方案**：wrapper 把 `--output-file` 写出的 execution info JSON 解析为 `session.json`，并把 stdout 收集为 `final_message.md`。
- **原因**：官方 non-interactive help 已公开 `--output-file`，且 bundle 实现表明其中稳定包含 `session-id` / `conversation-id` / token usage 等元数据。
- **备选**：从 stdout/stderr 正则抓 session id。
- **不选原因**：脆弱且更容易受文案或 ANSI 输出影响。

### 5. 项目级 `.iflow/settings.json` 只存非敏感默认项
- **方案**：新增 `.iflow/settings.json`，只声明 `contextFileName` 指向 `AGENTS.md`（可用数组形式兼容未来扩展）。
- **原因**：让 iFlow 在项目内天然复用现有协作说明，不引入第二套重复 memory 文件。
- **备选**：新增仓库内 `IFLOW.md`。
- **不选原因**：会复制一份本已由 `AGENTS.md` 维护的协作规范，后续容易漂移。

### 6. streaming 采取“line-based best effort + artifact final override”
- **方案**：chat manager 为 `iflow` 增加一个 plain-text parser，把 stdout 每行视为 assistant delta；turn 完成后仍以 `final_message.md` 和 `session.json` 作为最终结果。
- **原因**：最小改动即可给前端增量反馈，同时保证最终结果以 artifacts 为准。
- **备选**：完全不做 streaming，只等待 turn 完成。
- **不选原因**：用户体验会显著落后于现有 CLI tool。

## Risks / Trade-offs

- **[Risk] iFlow non-interactive stdout 含额外提示信息** → 通过 wrapper 剥离 ANSI，并在最终结果阶段以 `final_message.md` 为准；必要时后续再增加更细过滤。
- **[Risk] `--resume` 的交互行为在版本升级后变化** → 保留现有通用 fallback：native resume 失败后自动 reconstructed prompt 重试。
- **[Risk] 给所有 CLI 注入 source env 可能引入意外副作用** → 只注入与已解析 `model_id` 匹配的 provider env，并保持 Codex / Claude wrapper 对多余 env 完全忽略。
- **[Risk] repo 内新增 `.iflow/settings.json` 改变部分开发者本地行为** → 文件只包含 `contextFileName` 这类非敏感默认项，不覆盖认证方式与密钥。

## Migration Plan

1. 新增 OpenSpec delta specs 与任务列表。
2. 在隔离 worktree 中实现 `iflow` tool / expert / wrapper / env 注入 / UI 文案更新。
3. 补单测，优先验证 config、expert resolve、chat runtime patch 与 parser 行为。
4. 运行针对性测试。
5. 更新 `PROJECT_STRUCTURE.md`。
6. 完成后执行 archive，把 delta specs 合回基线。

## Open Questions

- 暂无阻塞性问题；若后续验证发现 iFlow 的 stdout 结构比预期更复杂，可在下一次迭代中把 line-based parser 升级为 JSON/marker-aware parser。
