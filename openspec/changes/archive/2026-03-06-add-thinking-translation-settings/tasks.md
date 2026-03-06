## 1. OpenSpec and config groundwork

- [x] 1.1 Add `thinking-translation` capability and update `ui` / `chat-session-memory` delta specs
- [x] 1.2 Extend `backend/internal/config/config.go` with `basic` settings schema and translation settings types
- [x] 1.3 Implement basic settings validation and LLM-settings-linked cleanup helpers

## 2. Backend API and runtime translation

- [x] 2.1 Add `GET/PUT /api/v1/settings/basic` handlers and register routes
- [x] 2.2 Extend expert resolve metadata so chat can match configured target model ids
- [x] 2.3 Implement buffered thinking translation pipeline in `backend/internal/chat/` with ordered delta events and failure fallback
- [x] 2.4 Extend chat completed payload with translated reasoning fields

## 3. Frontend settings and chat rendering

- [x] 3.1 Add daemon client types/functions for basic settings
- [x] 3.2 Add `BasicSettingsTab` and wire it into `SettingsDialog` as the leftmost tab
- [x] 3.3 Extend chat store for translated reasoning state and translation failure state
- [x] 3.4 Update `ChatSessionsPage` to render translated Chinese reasoning or raw fallback

## 4. Tests, docs, and archive

- [x] 4.1 Add Go tests for basic settings validation/API and chat translation behavior
- [x] 4.2 Update `PROJECT_STRUCTURE.md` for the new settings API/component responsibilities
- [x] 4.3 Run targeted tests plus UI build
- [x] 4.4 Archive the completed OpenSpec change into `openspec/changes/archive/`
