## 1. Backend SSE Stream

- [x] 1.1 Add an in-process SSE broker for Repo Library events
- [x] 1.2 Add `GET /api/v1/repo-library/stream` handler (SSE + keep-alive)
- [x] 1.3 Broadcast `repo_library.analysis.updated` on queued/running/succeeded/failed transitions

## 2. UI Live Updates + Indicators

- [x] 2.1 Render status indicator icons in `RepoLibrarySidebarRepositoryList`
- [x] 2.2 Subscribe to SSE on Repo Library routes and refresh repositories list on events
- [x] 2.3 Debounce refresh to avoid bursty updates

## 3. Verification

- [x] 3.1 `go test ./...`
- [x] 3.2 `pnpm -C ui run build`
- [ ] 3.3 Manual: start an analysis and observe sidebar indicator changes without manual refresh
