## 1. Backend Storage and Schema

- [x] 1.1 Bump SQLite schema version and add chat tables/indexes (`chat_sessions`, `chat_messages`, `chat_anchors`, `chat_compactions`)
- [x] 1.2 Implement store layer CRUD for sessions/messages/anchors/compactions
- [x] 1.3 Add migration/store unit tests for chat persistence and query ordering

## 2. Chat Domain and SDK Integration

- [x] 2.1 Add chat manager service for turn lifecycle (append user msg -> stream assistant -> persist result)
- [x] 2.2 Add context estimator and threshold-based auto-compaction pipeline with metadata records
- [x] 2.3 Integrate provider anchors for OpenAI (`previous_response_id`) and Anthropic (`container`) with fallback behavior

## 3. API and Realtime Events

- [x] 3.1 Add `/api/v1/chat/*` HTTP routes and handlers (create/list/messages/turns/compact/fork)
- [x] 3.2 Emit and document `chat.turn.*` and `chat.session.compacted` WS events through existing hub
- [x] 3.3 Add API-level tests for chat endpoints and streaming-safe persistence paths

## 4. Frontend Chat Experience

- [x] 4.1 Add chat route/page and topbar entry for `#/chat`
- [x] 4.2 Add frontend daemon client methods and zustand store for session/message state
- [x] 4.3 Wire WS chat events into incremental UI rendering and add compact/fork actions

## 5. Validation and Documentation

- [x] 5.1 Run backend and frontend tests/build checks; fix regressions
- [x] 5.2 Update `PROJECT_STRUCTURE.md` with chat module/file ownership and API location
- [x] 5.3 Archive change with `/opsx:archive` after tasks and validations complete
