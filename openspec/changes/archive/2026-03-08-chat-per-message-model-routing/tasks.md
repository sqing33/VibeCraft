## 1. Session / API / Runtime Chain

- [x] 1.1 为 `chat_sessions` 新增 `reasoning_effort` 字段，并完成 migrate / compatibility repair
- [x] 1.2 更新 store 的 create / list / get / update defaults / fork 链路，持久化 session 默认 effort
- [x] 1.3 更新 chat API，请求体支持 `reasoning_effort` 并做枚举校验
- [x] 1.4 更新 chat manager，在成功 turn 后让 session 默认值跟随 last-used effort

## 2. Codex App Server Integration

- [x] 2.1 在线程配置中注入 `model_reasoning_effort`
- [x] 2.2 在 `turn/start` 中注入本条消息的 `effort`
- [x] 2.3 为 store 持久化与 Codex runtime settings 增加针对性测试

## 3. Chat Composer UI

- [x] 3.1 前端类型与状态链路支持 `reasoning_effort`
- [x] 3.2 将输入区改为左侧大输入框 + 右侧窄控制栏
- [x] 3.3 右侧控制栏改为三行：CLI、模型、思考程度+上传/发送
- [x] 3.4 非 Codex 运行时仍显示思考程度控件但禁用

## 4. Verification

- [ ] 4.1 手工验证 `#/chat`：Codex 运行时可切换 low / medium / high / xhigh，并随下一条消息生效
- [ ] 4.2 手工验证 `#/chat`：切到非 Codex 运行时后，思考程度控件保持可见但禁用
- [ ] 4.3 手工验证输入区高度与右侧控制栏对齐，左侧输入框占满剩余高度
