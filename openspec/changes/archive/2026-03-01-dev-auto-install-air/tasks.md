## 1. 规范与文档

- [x] 1.1 更新 `dev-hot-reload` delta spec：补充缺失 Air 时自动安装的行为与场景
- [x] 1.2 更新 `README.md`：说明 `scripts/dev.sh` 会在缺失 Air 时自动安装，以及如何禁用

## 2. 脚本实现

- [x] 2.1 更新 `scripts/dev.sh`：`air` 缺失时尝试 `go install github.com/air-verse/air@latest`
- [x] 2.2 处理 PATH：确保 `go install` 后当前脚本进程能找到 `air`（`GOBIN`/`GOPATH/bin`）
- [x] 2.3 安装失败时安全回退：打印提示并降级为 `go run`
- [x] 2.4 保持 `VIBECRAFT_NO_AIR=1` 禁用语义与现有进程回收语义不变

## 3. 验证

- [x] 3.1 `bash -n scripts/dev.sh` 通过
- [x] 3.2 本地验证：临时移除/屏蔽 `air` 后运行 `./scripts/dev.sh`，确认会自动安装并使用 Air
