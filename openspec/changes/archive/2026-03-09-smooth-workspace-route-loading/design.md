## Context

The shared workspace shell solved full-page chrome remounts, but most route data still lives in page-local `useState`. That means route switches still start from empty arrays or `null` detail state until fetches resolve.

Repo Library repeats repository-list fetching across list/search/detail pages, and Orchestration list/detail pages separately fetch recent orchestrations. These are ideal candidates for lightweight zustand-based UI caches because the app already uses zustand for other cross-route UI state.

## Goals / Non-Goals

**Goals:**
- Keep prior workspace content visible during route switches and background refreshes.
- Show a subtle loading indication without replacing content with blank states.
- Reuse existing fetch functions and avoid introducing a new query framework.

**Non-Goals:**
- Change backend APIs or response shapes.
- Add complex page transition animations or motion-heavy route choreography.
- Rework Chat data flow beyond its current persisted store behavior.

## Decisions

### 1. Use lightweight zustand UI caches
Repo Library and Orchestration pages will use dedicated zustand stores to cache lists, search state, and detail snapshots across route unmounts.

### 2. Distinguish initial loading from refreshing
Pages will show full skeletons only when no cached data exists. Once any cache exists, refreshes will keep old content on screen and render a translucent loading veil.

### 3. Keep fetch orchestration in pages
Page modules keep their existing fetch logic and domain shaping. The new stores only preserve UI-facing state across route changes.

## Risks / Trade-offs

- **[Risk] Cached stale content may briefly mismatch the newly selected route** → Mitigation: show a loading veil and update active sidebar selection immediately.
- **[Risk] Nested repo detail state is broader than list caches** → Mitigation: cache the full rendered repo-detail bundle per repository ID.
- **[Risk] Store complexity grows** → Mitigation: keep stores narrowly scoped to workspace transition state and reuse existing types.
