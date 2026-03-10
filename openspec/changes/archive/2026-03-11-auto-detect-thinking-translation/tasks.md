# Tasks

## 1. OpenSpec

- [x] 1.1 补齐 proposal / design / delta specs

## 2. Backend config and API

- [x] 2.1 简化 `basic.thinking_translation` 为仅依赖 `model_id`
- [x] 2.2 更新 `/api/v1/settings/basic` 的读写结构与兼容逻辑
- [x] 2.3 调整 thinking translation spec 构建逻辑，不再依赖目标模型列表

## 3. Runtime auto detection

- [x] 3.1 为 thinking translation runtime 增加 entry 级自动判断状态
- [x] 3.2 实现“中文主导则跳过翻译”的本地启发式判断
- [x] 3.3 让 `thinking_translation_applied` 仅表示实际发生翻译

## 4. UI

- [x] 4.1 精简基本设置页为仅选择翻译模型
- [x] 4.2 更新前端类型定义与保存请求结构

## 5. Verification

- [x] 5.1 追加配置与 API 测试
- [x] 5.2 追加自动判断与 applied 语义测试
- [x] 5.3 运行 `go test` 与 `npm run build`
