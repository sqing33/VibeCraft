## Why

The current iFlow integration incorrectly reuses the generic OpenAI-compatible model/source mechanism. In practice this causes unsupported-model failures, unusable auth flows, and a broken user experience. The user clarified that iFlow must only use its official authentication and its own model catalog.

## What Changes

- Treat `iFlow CLI` as a special CLI runtime instead of a generic OpenAI-compatible tool.
- Add dedicated iFlow settings in `CLI 工具` for:
  - official auth mode (`网页登录` / `API Key`)
  - official base URL
  - iFlow model list and default model
- Add daemon-managed browser auth sessions that:
  - launch real `iflow`
  - auto-select the first auth option
  - capture the real OAuth URL printed by the terminal
  - accept pasted authorization codes and finish login
- Persist browser auth inside a daemon-managed iFlow home so vibe-tree can reuse it without depending on the user’s global `~/.iflow`.
- Inject iFlow runtime data per turn:
  - official auth env
  - selected iFlow model
  - effective MCP servers
  - effective skill instructions
- Update Chat / Repo Library model selectors so iFlow uses the iFlow card model list instead of the shared LLM model pool.

## Impact

- Affects backend config, settings APIs, CLI runtime wrapper, chat runtime preparation, Repo Library runtime selection, and system settings UI.
- Preserves existing Codex / Claude behavior.
- Restores a complete iFlow path from login to chat / repo-analysis execution.
