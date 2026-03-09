## Why

The current Chat, Orchestrations, and Repo Library routes each own their own immersive left-right layout. When the user switches among the three primary product lanes, the whole page chrome remounts, which feels like a full-page refresh instead of a React-style content switch.

We need a shared workspace shell now because the left rail design, status controls, and right content frame have already converged visually, and users now expect the left rail to stay fixed while only the lane-specific content changes.

## What Changes

- Introduce a shared workspace app shell that owns the persistent left rail chrome and the bordered right content frame for top-level workspace routes.
- Move Chat, Orchestrations, and Repo Library routes to render lane-specific sidebar content and right-panel content inside that shared shell instead of each page rendering its own outer layout.
- Standardize the shared shell visual contract already requested by the user, including integrated left rail styling, `5px` outer padding, and a `10px` rounded right panel.
- Preserve current route-level behaviors inside each lane, including chat session management, repo-library repository/search/detail flows, and orchestration list/detail flows.

## Capabilities

### New Capabilities
- `workspace-app-shell`: A persistent workspace shell for top-level product lanes, with fixed global navigation and shared status/actions.

### Modified Capabilities
- `chat-page-immersive-layout`: Chat must render as the chat lane inside the shared workspace shell instead of owning an isolated outer layout.
- `repo-library-ui`: Repo Library list, search, and detail routes must render inside the shared workspace shell with a persistent repository sidebar lane.
- `project-orchestration-ui`: Orchestration list and detail routes must render inside the shared workspace shell with a persistent recent-orchestrations sidebar lane.

## Impact

- Affected frontend routing and layout files: `ui/src/App.tsx`, `ui/src/app/routes.ts`, and new or refactored shared shell components.
- Affected product-lane pages: Chat, Repo Library, and Orchestrations UI pages.
- Affected documentation: `PROJECT_STRUCTURE.md` and related OpenSpec specs for chat, repo library, and orchestration UI.
