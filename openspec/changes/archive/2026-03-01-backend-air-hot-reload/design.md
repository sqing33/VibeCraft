## Context

当前本地开发通过 `scripts/dev.sh` 在后台 `go run ./cmd/vibecraft-daemon` 启动后端，同时在前台启动 UI（Vite dev server）。后端代码改动需要手动重启 daemon，导致联调效率偏低。

目标是在不改变 daemon 运行时行为/对外 API 的前提下，引入 Air（Go 热重载工具）用于开发期自动重建与重启。

约束与现状：
- `scripts/dev.sh` 需要继续“一键启动”体验
- 后端进程需要能在脚本退出时被正确回收（现有 trap + kill）
- Air 为开发工具，不应成为运行时依赖；安装方式以 `go install` 为主

## Goals / Non-Goals

**Goals:**
- 在 `backend/` 目录内提供可用的 Air 配置，支持监听 Go 文件变更并自动重启 daemon
- `scripts/dev.sh` 默认优先用 Air 启动后端；若 Air 不存在则降级为现有 `go run`
- 开发文档明确安装、启动方式与常见排障路径

**Non-Goals:**
- 不改变 daemon 的 API、配置语义、存储结构或运行时逻辑
- 不为生产/发布流程引入 Air（不参与 CI/构建产物）
- 不引入复杂的多进程编排器（如 `docker compose`/`foreman` 等）

## Decisions

1. **Air 配置放在 `backend/.air.toml`**
   - 让 Air 的工作目录与 watch root 保持在后端子树内，避免误监听 `ui/`、`desktop/` 等目录
   - 与当前 `scripts/dev.sh` 的 `cd backend` 行为一致
   - 备选：仓库根 `.air.toml`（放弃：需要更复杂的 include/exclude 规则）

2. **Air 通过 `go build` 生成临时二进制再运行**
   - 使用 `go build -o ./tmp/vibecraft-daemon ./cmd/vibecraft-daemon`，运行 `./tmp/vibecraft-daemon`
   - 相比 `go run`：更接近真实运行方式，启动更稳定，且避免每次运行重复编译所有包的临时缓存行为不确定

3. **脚本“优先 Air、缺失降级”**
   - `scripts/dev.sh` 在启动后端前检测 `air` 是否在 PATH（`command -v air`）
   - 若存在则运行 `air -c .air.toml`（在 `backend/` 目录内），否则继续 `go run`
   - 保留一个显式开关以禁用热重载（例如 `VIBECRAFT_NO_AIR=1`），便于排障与最小依赖运行

## Risks / Trade-offs

- [Risk] 开发者机器未安装 Air 导致脚本启动失败 → Mitigation：脚本自动降级，并在日志里提示安装命令
- [Risk] watch 规则过宽造成频繁重启 → Mitigation：限制 root 在 `backend/`，排除 `tmp/`、`vendor/` 等目录
- [Trade-off] 默认启用 Air 会引入额外输出与一次性安装成本 → Mitigation：文档说明 + 可通过环境变量禁用
