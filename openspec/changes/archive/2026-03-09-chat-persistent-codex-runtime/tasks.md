## 1. Spec & Design

- [x] 1.1 Write proposal/design for warm Codex app-server reuse
- [x] 1.2 Add spec deltas for chat resume and CLI runtime lifecycle

## 2. Backend Runtime Pool

- [x] 2.1 Add session-scoped Codex runtime pool with runtime signature matching and idle eviction
- [x] 2.2 Refactor Codex chat turn flow to reuse warm client, and cold-resume only when needed
- [x] 2.3 Invalidate warm runtime on archive and close pool on daemon shutdown

## 3. Validation

- [x] 3.1 Add/update backend tests for warm reuse, rebuild-on-config-change, and archive invalidation
- [x] 3.2 Run targeted backend tests
