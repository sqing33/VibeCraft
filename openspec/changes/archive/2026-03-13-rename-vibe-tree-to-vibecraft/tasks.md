## 1. Rename Surfaces

- [x] 1.1 Update root docs, primary docs, UI copy, desktop labels, and product-facing prompts from `vibe-tree` to `VibeCraft`
- [x] 1.2 Replace runtime-facing identifiers from `vibe-tree` to `vibecraft` across executable names, config/data/log defaults, and environment variable prefixes

## 2. Runtime Compatibility

- [x] 2.1 Add compatibility or migration logic so existing `vibe-tree` config and data remain readable after the rename
- [x] 2.2 Update scripts, build entry points, and managed runtime config generation to use new names without breaking existing flows

## 3. Repository and Verification

- [ ] 3.1 Update source paths, module/import references, and final working directory name to the new project name
- [x] 3.2 Run targeted build and test verification for backend, frontend, and affected tooling; then update change tracking and archive readiness notes
