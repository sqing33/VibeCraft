## 1. Database Migration

- [x] 1.1 Bump SQLite schemaVersion to 3 and add migrateV3 for chat_messages metadata columns (expert_id/provider/model)
- [x] 1.2 Update store chat message scan/append paths to read/write new columns and tolerate NULL for legacy rows
- [x] 1.3 Add/adjust store tests covering migration + NULL compatibility

## 2. Backend API & Chat Manager

- [x] 2.1 Extend turn request to accept optional expert_id and route to expert.Resolve for that turn
- [x] 2.2 Persist per-message expert/provider/model metadata for both user and assistant messages
- [x] 2.3 Implement anchor safety: only reuse anchor when provider+model matches session defaults; bypass otherwise; keep existing invalid-anchor retry fallback
- [x] 2.4 Update WebSocket payload for chat.turn.started to include expert_id/provider/model
- [x] 2.5 Update API integration tests to cover per-turn expert override and session defaults following last-used expert

## 3. LLM Compaction

- [x] 3.1 Implement LLM-first compaction summary generation with deterministic fallback on failure
- [x] 3.2 Ensure auto compaction and manual compact share the same LLM-first policy and still create compaction records
- [x] 3.3 Add tests validating compaction fallback does not fail the turn and summary is updated

## 4. Frontend (Chat UI)

- [x] 4.1 Extend daemon.ts ChatMessage types for expert_id/provider/model and extend postChatTurn to send expert_id
- [x] 4.2 Update chatStore.sendTurn to accept expertId and to tag the local optimistic user message with expert_id
- [x] 4.3 Add per-message Expert selector to ChatSessionsPage composer and send expert_id with turn requests
- [x] 4.4 Render per-message model identity (expert label + provider/model) in the chat transcript and streaming bubble

## 5. Manual Verification

- [ ] 5.1 Verify in `#/chat` that switching expert per message works, streaming labels reflect the selected expert, and refresh preserves history metadata
- [ ] 5.2 Verify long conversation triggers LLM compaction and continues correctly; manual compact works and emits feedback
