## Context

当前 `vibe-tree` 的 Expert 配置默认通过 `${ENV_VAR}` 从系统环境变量注入敏感信息（如 `ANTHROPIC_API_KEY`）。在本地开发时，用户需要手动 `export ANTHROPIC_API_KEY=...` 才能启动/执行 Anthropic provider 的 expert；否则在模板展开阶段会报 `missing env vars: ANTHROPIC_API_KEY`。

本变更希望让“把 key 放在仓库根目录 `.env`”成为一个可复现、低摩擦的开发体验，同时避免把 key 明文写入 `~/.config/vibe-tree/config.json`。

约束：
- 不引入 UI 配置/输入密钥能力（仅本地启动时读取 dotenv）。
- 日志必须避免泄露密钥内容（不打印 value）。
- 行为需要可禁用、可指定路径，避免误加载或对已有环境造成不可控影响。

## Goals / Non-Goals

**Goals:**
- daemon 启动时（`config.Load()` 之前）自动加载 dotenv，为后续 `${ENV_VAR}` 注入提供环境变量来源。
- 默认加载 repo root 的 `.env`（通过向上查找 `.git/` 定位 repo root）。
- 支持 `VIBE_TREE_DOTENV_PATH` 显式指定 dotenv 文件路径。
- 支持 `VIBE_TREE_DOTENV=0` 禁用加载。
- dotenv 的 key 写入进程环境变量时采用“覆盖”策略（`.env` 覆盖已有同名环境变量）。
- 提供单测覆盖关键路径与覆盖语义。

**Non-Goals:**
- 不支持读取 `.env.*` 多环境矩阵（如 `.env.local`/`.env.development`）。
- 不提供密钥加密、keyring 集成或 UI 输入/存储。
- 不改变 expert 模板展开与 `missing env vars` 的错误语义（dotenv 只是补齐环境变量来源）。

## Decisions

### 1) 在 Go daemon 内实现 dotenv 加载（而非仅脚本）

选择在 `backend/cmd/vibe-tree-daemon/main.go` 中调用 dotenv loader，确保：
- 用户直接运行 `go run ./cmd/vibe-tree-daemon` 也能生效（不依赖 `scripts/*.sh`）。
- 行为由后端掌控，便于测试与记录日志。

替代方案：仅在 `scripts/dev.sh`/`scripts/web.sh` 中 `source .env`。该方案对直接 `go run`/桌面壳启动不生效，且不易覆盖单测。

### 2) Repo root 定位方式：向上查找 `.git/`

默认行为需要在任意工作目录启动 daemon 都能找到仓库根（例如脚本在 `backend/` 目录运行）。实现上从 `os.Getwd()` 向上逐级查找是否存在 `.git` 目录，找到后使用 `<repoRoot>/.env`。

替代方案：硬编码相对路径（如 `../.env`）或要求从仓库根启动。可用性差且容易踩坑。

### 3) dotenv 解析库：使用 `github.com/joho/godotenv`

原因：
- 解析规则成熟，能处理常见 `.env` 语法（引号、注释、空行）。
- 实现成本低，减少自写解析器的边界问题。

### 4) 覆盖语义：`.env` 覆盖已有环境变量

按用户偏好，采用覆盖策略：读取到的每个 key 都执行 `os.Setenv(key, value)`。

替代方案：不覆盖（仅补齐缺失）。实现上需要判断 `os.LookupEnv` 后再决定写入；更安全但不符合本次选择。

### 5) 错误处理策略：尽量不阻断启动

- `.env` 不存在：静默跳过（info/debug）。
- `.env` 解析失败或读文件失败：记录 warn，并继续启动（用户仍可通过 `export`/系统 env 方式提供 key，或使用 `demo`/`bash` expert）。

## Risks / Trade-offs

- [dotenv 覆盖可能导致意外行为] → 提供 `VIBE_TREE_DOTENV=0` 禁用，提供 `VIBE_TREE_DOTENV_PATH` 显式指定；日志记录“是否加载/路径/keys 数量”便于排障。
- [repo root 查找在非 git 环境失败] → 若找不到 `.git/` 则默认不加载（或仅在 `VIBE_TREE_DOTENV_PATH` 指定时加载该文件）。
- [引入第三方依赖] → 选择小而稳定的 dotenv 解析库，并通过单测锁定核心行为。
