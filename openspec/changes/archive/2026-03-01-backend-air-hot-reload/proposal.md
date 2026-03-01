## Why

Backend 本地开发目前通过 `go run` 启动，改动 Go 代码后需要手动重启 daemon，反馈回路偏长。引入 Air 热重载可以显著提升后端迭代速度，并减少 UI/desktop 联调时的等待成本。

## What Changes

- 在 `backend/` 引入 Air 配置（`.air.toml`），支持监听 Go 源码变更并自动重建/重启 daemon
- 调整本地开发启动脚本（`scripts/dev.sh`）优先使用 Air 启动后端（并提供 Air 缺失时的降级路径）
- 补充开发文档：如何安装 Air、如何启动/排障热重载

## Capabilities

### New Capabilities
- `dev-hot-reload`: 定义使用 Air 对 Go daemon 进行热重载的本地开发工作流与约束（配置位置、启动方式、默认行为、降级策略）

### Modified Capabilities
- (none)

## Impact

- 影响本地开发体验与脚本：`scripts/dev.sh`、新增 `backend/.air.toml`
- 新增本地开发依赖：Air（通过 `go install` 安装；不引入到项目运行时依赖）
- 不改变 daemon 的对外 HTTP API 与运行时行为（仅开发启动方式变化）
