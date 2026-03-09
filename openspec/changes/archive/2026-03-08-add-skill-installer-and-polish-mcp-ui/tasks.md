## 1. Backend: Skill settings and install

- [x] 1.1 Extend `settings/skills` API to return discovered skills with persisted enabled state
- [x] 1.2 Add `PUT /api/v1/settings/skills` to persist per-skill enabled flags
- [x] 1.3 Add `POST /api/v1/settings/skills/install` for zip and folder uploads into `~/.codex/skills`
- [x] 1.4 Update runtime filtering so only enabled discovered skills are injected, then intersect with `expert.enabled_skills`
- [x] 1.5 Add/adjust Go tests for settings read-write, install flow, and runtime filtering

## 2. Frontend: MCP / Skill settings UX

- [x] 2.1 Refactor `SettingsDialog` so MCP / Skill tabs use fixed header + inner scroll area
- [x] 2.2 Rebuild `MCPSettingsTab` into a two-column card grid with compact CLI default toggles
- [x] 2.3 Remove MCP explanatory alerts and use Context7 JSON placeholder for new entries
- [x] 2.4 Rebuild `SkillSettingsTab` with per-skill enabled switch and add-skill upload flow
- [x] 2.5 Add daemon client APIs for saving skills and uploading skill packages

## 3. Docs and archive

- [x] 3.1 Update `PROJECT_STRUCTURE.md` for Skill install / settings responsibilities
- [x] 3.2 Sync `openspec/specs/mcp-skill-settings/spec.md` to reflect new behavior
- [x] 3.3 Archive the completed change
