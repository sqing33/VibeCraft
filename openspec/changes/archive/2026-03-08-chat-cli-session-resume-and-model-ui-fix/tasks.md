## 1. Schema & Contract

- [x] 1.1 Extend chat session persistence with `cli_tool_id`, `model_id`, and `cli_session_id`
- [x] 1.2 Extend CLI artifact parsing to read/write `session.json`

## 2. Wrapper Resume Support

- [x] 2.1 Update `codex_exec.sh` to capture `thread_id` and support `resume <session_id>`
- [x] 2.2 Update `claude_exec.sh` to capture `session_id` and support `-r <session_id>`

## 3. Chat Backend

- [x] 3.1 Update chat create-session/turn APIs to persist and reuse CLI session references
- [x] 3.2 Make chat turn use CLI resume first, then fallback to reconstructed prompt on failure
- [x] 3.3 Keep attachments / compaction / thinking translation compatible

## 4. Chat UI

- [x] 4.1 Fix new-session model selector so the selected model label is visible
- [x] 4.2 Fix composer model selector so the selected/current model label is visible

## 5. Validation

- [x] 5.1 Run backend tests
- [x] 5.2 Run UI build
