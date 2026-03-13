## 1. Backend pagination support

- [x] 1.1 Add store query for messages before a turn (`turn < before_turn`) while preserving ascending response order and attachments
- [x] 1.2 Extend `GET /api/v1/chat/sessions/:id/messages` to accept and validate `before_turn`
- [x] 1.3 Add API integration tests for `before_turn` pagination and invalid input

## 2. UI virtualized transcript rendering

- [x] 2.1 Add `react-virtuoso` dependency to UI
- [x] 2.2 Implement a virtualized chat transcript component (Virtuoso) that renders message bubbles and the pending assistant footer
- [x] 2.3 Implement “at bottom follow output / scroll-to-bottom button” behavior in the virtualized transcript

## 3. Infinite scroll + message loading semantics

- [x] 3.1 Extend `fetchChatMessages` to accept `before_turn` and keep current default `limit`
- [x] 3.2 Update chat store message loading to merge/dedupe messages (avoid overwriting already-loaded older history)
- [x] 3.3 Add `loadOlderMessages` flow triggered on transcript top reach and prepend results without scroll jump

## 4. Verification

- [ ] 4.1 Manually verify long-session performance (no UI freeze) and prepend scrolling stability
- [ ] 4.2 Manually verify attachments preview + Markdown rendering remain correct
- [x] 4.3 Run backend + UI builds/tests (`go test ./...`, `pnpm -C ui build`)
