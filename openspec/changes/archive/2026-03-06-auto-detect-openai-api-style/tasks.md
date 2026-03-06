## 1. OpenSpec and config groundwork

- [x] 1.1 Add `openai-api-style-routing` capability and update `llm-settings` / `llm-test` delta specs
- [x] 1.2 Extend `LLMModelConfig` with hidden OpenAI API style metadata fields
- [x] 1.3 Implement preserve/invalidate helpers for hidden API style metadata during `PUT /settings/llm`

## 2. OpenAI compatibility layer

- [x] 2.1 Add `backend/internal/openaicompat/` with API style enum, endpoint-mismatch classification, probing, and text-generation helpers
- [x] 2.2 Support `responses -> chat/completions` probing with single-model persistence hooks
- [x] 2.3 Support one-time re-probe when saved style no longer matches the gateway

## 3. Integrate auto-detect into runtime

- [x] 3.1 Update `POST /api/v1/settings/llm/test` to probe and persist style for saved OpenAI models
- [x] 3.2 Update Chat OpenAI path to use the compat layer and degrade unsupported Responses-only features explicitly
- [x] 3.3 Update thinking translation OpenAI path to use the compat layer for plain text translation
- [x] 3.4 Update SDK runner OpenAI path to use the compat layer and reject strict structured output on `chat/completions`

## 4. Tests, docs, and archive

- [x] 4.1 Add config/openaicompat/api/chat/runner tests for probing, persistence, invalidation, and degradation
- [x] 4.2 Update `PROJECT_STRUCTURE.md` for the new OpenAI compat layer and hidden model metadata responsibility
- [x] 4.3 Run targeted backend tests and UI build
- [x] 4.4 Archive the completed OpenSpec change into `openspec/changes/archive/`
