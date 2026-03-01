## Why

当前 `scripts/dev.sh` 在缺失 `air` 时会降级为 `go run`，但这会让首次上手的开发者失去后端热重载能力。让脚本在缺失 Air 时自动安装，可以进一步降低本地开发门槛并统一团队开发体验。

## What Changes

- `scripts/dev.sh` 在 `air` 不存在且未显式禁用时，自动执行 `go install github.com/air-verse/air@latest` 安装 Air，然后再以 Air 启动后端
- 更新开发文档：说明脚本会自动安装 Air；以及如何通过环境变量禁用 Air
- 更新 `dev-hot-reload` 规范：补充“缺失 Air 时自动安装”的要求与场景

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `dev-hot-reload`: dev.sh 在缺失 Air 时应自动安装 Air，并在可用后使用 Air 启动后端

## Impact

- 影响本地开发脚本行为：首次运行可能触发网络下载与 `go install`
- 不影响 daemon 的对外 API 与运行时行为（仅开发工具链变化）
