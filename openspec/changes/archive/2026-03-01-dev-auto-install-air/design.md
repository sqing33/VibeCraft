## Context

目前仓库已经通过 `backend/.air.toml` 与 `scripts/dev.sh` 支持后端热重载：当 `air` 存在时使用 Air，否则降级为 `go run`。但对新环境来说，缺失 Air 很常见，导致“开箱即用”的热重载体验缺失。

本变更让 `scripts/dev.sh` 在满足条件时自动安装 Air，减少人工步骤。

约束：
- Air 仍是开发期工具，不应成为运行时依赖
- 安装方式优先 `go install github.com/air-verse/air@latest`（可重复执行，行为可预测）
- 需要兼容 `GOBIN`/`GOPATH` 安装路径不在当前 `PATH` 的情况
- 保留 `VIBE_TREE_NO_AIR=1` 作为显式禁用开关（用于排障/最小依赖）

## Goals / Non-Goals

**Goals:**
- `air` 缺失时，`scripts/dev.sh` 自动安装 Air，并在安装成功后使用 Air 启动后端
- 安装失败时安全降级为 `go run`，并给出可操作的提示
- 文档同步更新：自动安装行为、禁用方式

**Non-Goals:**
- 不引入 OS 包管理器（brew/apt/choco）安装逻辑
- 不缓存/固定 Air 版本（仍使用 `@latest`）
- 不改变 Air 配置文件或 daemon 逻辑

## Decisions

1. **使用 `go install` 作为“自动下载/安装”方式**
   - 这是 Go 官方推荐的安装方式，自动下载源码并构建安装
   - 备选：直接下载 release 二进制（放弃：需要按 OS/arch 分支与校验，复杂度更高）

2. **安装前后处理 PATH**
   - 在脚本内读取 `go env GOBIN` 或 `go env GOPATH`，将对应 `bin` 临时加入 `PATH`
   - 确保 `go install` 后能在同一次脚本执行中找到 `air`

3. **失败回退策略**
   - 若 `go` 不存在、网络失败、或 `go install` 失败：打印提示并回退到 `go run`

## Risks / Trade-offs

- [Risk] 自动安装会触发网络下载，首次启动变慢 → Mitigation：仅在缺失 Air 时执行，并输出明确日志
- [Risk] 开发者不希望脚本自动改动本机 Go 工具链环境 → Mitigation：提供 `VIBE_TREE_NO_AIR=1` 禁用开关
