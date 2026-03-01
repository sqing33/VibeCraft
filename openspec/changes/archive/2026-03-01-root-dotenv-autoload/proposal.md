## Why

本地开发时，`master`/`claudecode` 等 Anthropic expert 依赖 `ANTHROPIC_API_KEY`。目前需要在启动前手动 `export`，否则会在 expert 解析阶段报 `missing env vars: ANTHROPIC_API_KEY`，影响开发体验与可复现性。

## What Changes

- daemon 启动时自动加载 dotenv：
  - 默认从仓库根目录 `.env` 读取（通过向上查找包含 `.git/` 的目录定位 repo root）。
  - 支持 `VIBE_TREE_DOTENV_PATH` 指定 dotenv 文件路径。
  - 支持 `VIBE_TREE_DOTENV=0` 禁用自动加载。
- dotenv 的键值写入进程环境变量（`os.Setenv`），用于后续 `${ENV_VAR}` 注入（例如 `ANTHROPIC_API_KEY`）。
- 记录启动日志（仅路径与 key 数量，不打印 value）。
- 提供根目录 `.env.example` 与根目录 `.gitignore`（忽略 `.env`），并更新 README 的开发配置说明。
- 增加 Go 单测覆盖：禁用开关、repo root 查找、路径指定、覆盖语义。

## Capabilities

### New Capabilities

- `dotenv`: daemon 启动时从 repo root/指定路径加载 `.env`，并将键值注入到进程环境变量中（含禁用/覆盖策略与日志约束）。

### Modified Capabilities

<!-- none -->

## Impact

- 后端启动入口：`backend/cmd/vibe-tree-daemon/main.go` 将在 `config.Load()` 前增加 dotenv 加载逻辑。
- 新增后端内部模块：`backend/internal/dotenv/`。
- Go 依赖新增：用于解析 `.env` 文件的库。
- 仓库根新增文件：`.gitignore`、`.env.example`；README 增补开发说明。
