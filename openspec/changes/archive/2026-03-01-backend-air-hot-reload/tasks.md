## 1. Air 集成（backend）

- [x] 1.1 新增 `backend/.air.toml`（watch 仅覆盖 `backend/`，排除 `tmp/` 等目录）
- [x] 1.2 确认 Air 以 `go build` 产出临时二进制并运行（如 `./tmp/vibecraft-daemon`）

## 2. 本地开发脚本调整

- [x] 2.1 更新 `scripts/dev.sh`：默认优先使用 `air` 启动后端（`command -v air` 检测）
- [x] 2.2 增加显式禁用开关（`VIBECRAFT_NO_AIR=1`）并确保会降级为 `go run`
- [x] 2.3 保持现有进程回收语义：脚本退出时能正确停止后端进程（Air 或 go run）

## 3. 文档与验收

- [x] 3.1 更新 `README.md`：说明 Air 安装方式（`go install ...`）、默认启用逻辑、禁用方法与常见排障
- [x] 3.2 本地验证：修改任意 `backend/internal/**.go` 后 daemon 自动重启且 UI 可继续联调
