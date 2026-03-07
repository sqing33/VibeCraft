## 1. Wrapper Event Contract

- [x] 1.1 Define normalized stdout event shape for chat wrappers
- [x] 1.2 Update Codex wrapper to emit incremental assistant/session events
- [x] 1.3 Update Claude wrapper to emit incremental assistant/thinking/session events

## 2. Chat Backend Streaming

- [x] 2.1 Refactor `runCLITurn()` to consume CLI output incrementally instead of `ReadAll`
- [x] 2.2 Map normalized wrapper events into `chat.turn.delta` / `chat.turn.thinking.delta` / completion updates
- [x] 2.3 Keep `final_message.md/summary.json` as final fallback for turn completion

## 3. Chat UI

- [x] 3.1 Ensure pending assistant bubble updates incrementally from CLI deltas
- [x] 3.2 Render available thinking/progress events during CLI execution

## 4. Validation

- [x] 4.1 Run backend tests
- [x] 4.2 Run UI build
