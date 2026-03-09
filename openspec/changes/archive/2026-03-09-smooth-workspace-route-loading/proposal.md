## Why

Workspace route switching currently remounts page-level data state for Repo Library and Orchestrations. Even though the shared shell stays mounted, users still see brief empty content before fresh data arrives.

We need smoother transitions so navigation feels continuous and intentional rather than flashing to empty states during every route change.

## What Changes

- Add client-side workspace UI caches for shared left-list and right-panel data used by Repo Library and Orchestrations.
- Change workspace routes to use stale-while-revalidate behavior: keep cached content visible while background refresh runs.
- Add a lightweight loading veil for sidebar and content regions so users see refreshing feedback without losing prior content.
- Preserve explicit full skeletons only for true first-load states with no cached content.

## Capabilities

### New Capabilities
- `workspace-loading-transitions`: Workspace routes provide cached, non-empty visual transitions during route switches and background refreshes.

### Modified Capabilities
- `workspace-app-shell`: Shared workspace routes surface a non-disruptive refreshing state instead of flashing empty content during route transitions.
- `repo-library-ui`: Repo Library pages preserve cached repository/search/detail content while background data refreshes.
- `project-orchestration-ui`: Orchestration pages preserve cached recent-list and detail content while background data refreshes.

## Impact

- Affected frontend stores and workspace pages under `ui/src/stores/` and `ui/src/app/pages/`.
- Affected workspace loading visuals in the shared shell and page-level content sections.
- Affected OpenSpec baselines for workspace shell, Repo Library UI, and Project Orchestration UI.
