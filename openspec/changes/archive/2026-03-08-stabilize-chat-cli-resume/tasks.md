## 1. Store & Migration

- [x] 1.1 Ensure new databases create `chat_sessions` with CLI session columns
- [x] 1.2 Ensure old databases add missing chat CLI columns during migration

## 2. Wrapper Resume Stability

- [x] 2.1 Harden Codex wrapper session extraction and resume invocation
- [x] 2.2 Harden Claude wrapper session extraction and resume invocation

## 3. Chat Runtime

- [x] 3.1 Make `runCLITurn()` prefer resume, then fallback once to reconstructed prompt
- [x] 3.2 Persist refreshed `cli_session_id` after successful turns

## 4. Chat UI

- [x] 4.1 Fix new-session model selector display
- [x] 4.2 Fix composer model selector display

## 5. Validation

- [x] 5.1 Run backend tests
- [x] 5.2 Run UI build
