## 1. Backend Normalization

- [x] 1.1 在 LLM settings 保存与专家镜像链路中统一将模型 `id/model` 归一化为小写
- [x] 1.2 为设置保存与大小写重复场景补充后端测试

## 2. Settings UI Restructure

- [x] 2.1 将模型设置从独立区块改为嵌入 API 源卡片，并把 SDK 选择上移到 Source 级别
- [x] 2.2 在 API Key 下新增模型列表编辑区，支持新增、删除、测试模型
- [x] 2.3 保留模型显示大小写，但保存与测试时统一使用小写模型标识

## 3. Verification

- [x] 3.1 运行相关测试与前端构建，确认设置保存、测试与专家刷新正常
