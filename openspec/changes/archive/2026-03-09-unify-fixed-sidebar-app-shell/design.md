## Context

The UI already uses an immersive left-right pattern for Chat, Repo Library, and Orchestrations, but each route currently renders its own full outer layout. That duplicates the same product navigation and system-status chrome across multiple components and causes visible remounting when users switch among top-level lanes.

This change is cross-cutting because it affects application routing, shared UI chrome, and three separate product areas that already have their own stateful page logic.

## Goals / Non-Goals

**Goals:**
- Keep the left rail chrome mounted while switching among Chat, Orchestrations, and Github 知识库.
- Let only the lane-specific sidebar body and right-side content outlet change with route navigation.
- Preserve the user-requested immersive visual rules: integrated left rail, `5px` outer padding, and a bordered right panel with `10px` radius.
- Minimize product-logic churn by reusing existing page state and store logic where possible.

**Non-Goals:**
- Redesign the legacy workflow detail surface at `#/workflows/:id`.
- Change backend APIs, routing formats, or existing domain data models.
- Rework route-local product behavior beyond what is required to render inside the shared shell.

## Decisions

### 1. Introduce a single shared workspace shell above top-level lane content
A new shared shell component will live above Chat, Repo Library, and Orchestrations route content. It will own:
- the global three-entry navigation at the top of the left rail,
- the shared health/status and utility actions at the bottom of the left rail,
- the bordered right content frame.

This keeps cross-lane chrome stable and removes duplicated shell code from lane pages.

**Alternatives considered:**
- Keep separate page-local shells and only align CSS tokens. Rejected because remounting behavior would remain.
- Put the shell inside each lane page and rely on memoization. Rejected because route switches still replace the shell subtree.

### 2. Treat the sidebar middle region and right panel as route-specific slots
The shared shell will accept lane-specific props for:
- active top nav entry,
- sidebar title/count/action/content,
- right-panel header meta/title/actions,
- right-panel body content.

This preserves the fixed left-rail frame while allowing Chat sessions, Repo Library repositories, and recent orchestrations to populate the middle region.

**Alternatives considered:**
- Force all lanes into one giant page component. Rejected because it would couple unrelated product logic and make state harder to maintain.

### 3. Keep domain state inside lane components and only lift layout ownership
Chat store usage, repo-library data fetching, and orchestration polling/log subscriptions will stay in their existing lane modules. The refactor will primarily extract or wrap view fragments so that the shared shell controls outer structure while each lane still owns its own domain state.

**Alternatives considered:**
- Move all route data fetching into `App.tsx`. Rejected because it would overload the app root and blur feature boundaries.

### 4. Extend the shared shell to detail routes that belong to the same lane
The shared shell will cover these routes:
- `#/chat`
- `#/orchestrations`
- `#/orchestrations/:id`
- `#/repo-library/repositories`
- `#/repo-library/repositories/:id`
- `#/repo-library/pattern-search`

This ensures that moving deeper within a lane also preserves the same left rail experience.

## Risks / Trade-offs

- **[Risk] Chat page extraction is state-heavy** → Mitigation: keep chat state/store logic in place and only extract sidebar/right-panel composition.
- **[Risk] Repo Library and Orchestration pages may duplicate slot-building code at first** → Mitigation: prefer a generic shell API and small lane-specific helper components rather than premature abstraction.
- **[Risk] Route transitions could accidentally break active-nav highlighting or lane-specific buttons** → Mitigation: centralize route-to-shell mapping in `App.tsx` and verify each top-level/detailed route explicitly.
- **[Risk] Visual regressions on spacing and rounding** → Mitigation: keep shared shell tokens aligned with the already-shipped immersive layout values and validate via build plus manual route checks.

## Migration Plan

1. Add the new shared workspace shell and route-to-shell mapping without changing non-workspace routes.
2. Refactor Chat, Repo Library, and Orchestration routes to provide shell slots instead of full outer shells.
3. Validate build output and manually check lane switching for stable chrome.
4. Archive the change after implementation so the new shell behavior becomes baseline spec truth.

## Open Questions

- No open product questions remain for this change; current behavior is fully driven by the user’s confirmed layout requirements.
