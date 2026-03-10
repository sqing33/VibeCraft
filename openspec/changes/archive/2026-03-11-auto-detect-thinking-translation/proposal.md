# auto-detect-thinking-translation

## Why

当前“思考过程翻译”需要用户手动选择“哪些模型需要翻译”。这会带来三个问题：
- 配置重复：同一套 runtime 模型已经在模型设置里维护，基本设置里再选一遍目标模型不合理。
- 容易失效：模型池调整后，`target_model_ids` 很容易残留失效引用。
- 交互负担高：用户真正关心的是“是否开启思考翻译”和“用哪个模型翻译”，不关心手工维护目标列表。

用户希望把这套逻辑改成：只配置一个翻译模型，然后在运行时根据思考过程内容自动判断是否需要翻译为中文。

## What Changes

- 简化基本设置中的思考过程翻译配置，只保留 `model_id`。
- 后端在 thinking translation runtime 中按 thinking entry 自动判断文本是否“中文主导”。
- 仅当内容不是中文主导时才调用翻译模型。
- `thinking_translation_applied` 改为表示“本轮实际发生了翻译”，而不是“存在翻译配置”。
- 基本设置页移除“需要翻译的 AI 模型”选择区，并将描述改为自动判断逻辑。

## Impact

- Backend:
  - `backend/internal/config/`
  - `backend/internal/api/`
  - `backend/internal/chat/`
- UI:
  - `ui/src/app/components/BasicSettingsTab.tsx`
  - `ui/src/lib/daemon.ts`
- Tests:
  - `backend/internal/config/basic_settings_test.go`
  - `backend/internal/api/settings_basic_test.go`
  - `backend/internal/chat/thinking_translation_test.go`

## Risks

- 本地语言判断是启发式规则，极少数中英混杂或日文/韩文场景可能存在边界误判。
- 需要保证误判“无需翻译”时，不会错误地把 `thinking_translation_applied` 标记为 true。
