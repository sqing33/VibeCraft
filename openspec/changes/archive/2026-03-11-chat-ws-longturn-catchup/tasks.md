## 1. Backend WebSocket Throughput

- [x] 1.1 Increase WS client send buffer and keep non-blocking broadcast semantics
- [x] 1.2 Batch-drain per-client send queue in `writePump`, emitting JSON array frames when >1 envelope is available
- [x] 1.3 Update backend WS integration tests to accept either single-envelope or array-of-envelopes frames

## 2. Frontend WebSocket Compatibility

- [x] 2.1 Update WS parsing to support single envelope or envelope array frames while preserving validation
- [x] 2.2 Update App WS handler to emit all parsed envelopes via the existing ws bus

## 3. Chat Long-Turn Catch-Up

- [x] 3.1 Make `loadTurns` return fetched turns so the chat page can reason about terminal vs running state during polling
- [x] 3.2 Add chat-page stale detection (pending turn + WS disconnected or quiet) and poll turns/messages to catch up without manual refresh

## 4. Verification

- [x] 4.1 Run `go test ./...` (backend) and fix any failures
- [x] 4.2 Run `npm run build` (ui) and fix any build/type errors
