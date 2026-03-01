## 1. Backend config & data model

- [x] 1.1 Extend `backend/internal/config/config.go` with `llm` settings schema (sources + models) and JSON types
- [x] 1.2 Implement LLM settings validation (IDs unique, provider allowed, URL format, model source reference)
- [x] 1.3 Implement config save utility (atomic write + `0600` perms + ensure config dir exists)
- [x] 1.4 Implement “mirror models to experts” helper (apply sources/models into `cfg.Experts` with provider-specific env keys)

## 2. Expert registry runtime reload

- [x] 2.1 Add RW lock to `backend/internal/expert/Registry` for concurrent Resolve/List access
- [x] 2.2 Add `Reload(cfg config.Config)` (or equivalent) to replace expert map at runtime

## 3. Daemon API: LLM settings

- [x] 3.1 Add `GET /api/v1/settings/llm` returning sources/models with masked key fields only
- [x] 3.2 Add `PUT /api/v1/settings/llm` to validate + persist + reload expert registry
- [x] 3.3 Wire routes in `backend/internal/api/api.go` and ensure errors are HTTP 400/500 as appropriate

## 4. Frontend: Settings Tabs shell

- [x] 4.1 Add shadcn `Tabs` component under `ui/src/components/ui/tabs.tsx`
- [x] 4.2 Refactor `ui/src/app/components/SettingsDialog.tsx` into tabbed layout with `连接与诊断` tab (existing content)

## 5. Frontend: LLM settings tab

- [x] 5.1 Add daemon client functions/types for LLM settings (`GET/PUT /api/v1/settings/llm`) in `ui/src/lib/daemon.ts`
- [x] 5.2 Implement `模型` tab UI: Sources editor (base URL + API key, masked display)
- [x] 5.3 Implement `模型` tab UI: Models editor (model name, source selection, provider selection: codex/openai vs claudecode/anthropic)
- [x] 5.4 On save success: toast + refresh experts store via `fetchExperts`

## 6. Tests & project hygiene

- [x] 6.1 Add Go unit tests for masking + validation + config save permissions
- [x] 6.2 Update `PROJECT_STRUCTURE.md` to include new API/config/responsibility entries if new modules/files added
