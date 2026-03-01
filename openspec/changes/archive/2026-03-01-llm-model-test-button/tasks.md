## 1. Backend: LLM test API

- [x] 1.1 Add `POST /api/v1/settings/llm/test` handler (validate provider/model/api_key/base_url)
- [x] 1.2 Implement SDK call for openai/anthropic with short prompt, short timeout, and output truncation
- [x] 1.3 Wire route in `backend/internal/api/api.go` and add Go test to ensure key is not leaked

## 2. Frontend: Model card test button

- [x] 2.1 Add daemon client function `postLLMTest` in `ui/src/lib/daemon.ts`
- [x] 2.2 Add `测试` button to each model card (left of delete) in `ui/src/app/components/LLMSettingsTab.tsx`
- [x] 2.3 Implement loading state and toast feedback for success/failure
