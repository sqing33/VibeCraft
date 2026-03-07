## 1. OpenSpec & Config Foundation

- [x] 1.1 Add `cli_tools` config schema with builtin `codex` and `claude`
- [x] 1.2 Add normalize/validate/reconcile logic for CLI tool settings and tool-model protocol binding
- [x] 1.3 Add `GET/PUT /api/v1/settings/cli-tools` and update daemon types

## 2. Runtime Resolution

- [x] 2.1 Refactor expert/runtime resolution so CLI tool + model become first-class selection inputs
- [x] 2.2 Stop exposing `llm.models` as the primary chat execution selection path
- [x] 2.3 Ensure workflow/orchestration primary CLI experts use tool-bound default models

## 3. Chat API & UI

- [x] 3.1 Extend chat create-session and turn APIs to accept `cli_tool_id` and `model_id`
- [x] 3.2 Update chat manager/session defaults to persist tool-first selections compatibly
- [x] 3.3 Replace chat expert selector with `CLI 工具 + 模型` selectors in the UI

## 4. Settings UI

- [x] 4.1 Add a dedicated `CLI 工具` tab for `Codex CLI` and `Claude Code`
- [x] 4.2 Update `LLMSettingsTab` to behave as a model pool, not a primary execution expert list
- [x] 4.3 Keep `ExpertSettingsTab` focused on persona/policy instead of model-as-expert

## 5. Validation & Cleanup

- [x] 5.1 Update backend tests for CLI tool settings and tool/model validation
- [x] 5.2 Run `go test ./...` in `backend/`
- [x] 5.3 Run `npm ci && npm run build` in `ui/`
- [x] 5.4 Sync baseline specs and archive the change
