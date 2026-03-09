## 1. Spec and config foundation

- [x] 1.1 Add `iflow` to default CLI tool configuration and built-in experts
- [x] 1.2 Extend CLI expert resolution to inherit selected model source runtime env
- [x] 1.3 Add or update unit tests covering `iflow` tool normalization and CLI runtime env injection

## 2. IFLOW wrapper and chat runtime

- [x] 2.1 Add `scripts/agent-runtimes/iflow_exec.sh` using official non-interactive flags
- [x] 2.2 Persist IFLOW `session-id` into `session.json` and final stdout into `final_message.md`
- [x] 2.3 Add IFLOW-specific streaming parsing and resume handling in chat runtime
- [x] 2.4 Add tests for IFLOW chat runtime behavior where practical

## 3. UI and project integration

- [x] 3.1 Update CLI tool settings and chat runtime UI copy to include iFlow CLI
- [x] 3.2 Add project-level `.iflow/settings.json` that points context loading at `AGENTS.md`
- [x] 3.3 Update `PROJECT_STRUCTURE.md` for new files and responsibilities

## 4. Validation and archive

- [x] 4.1 Run targeted backend and frontend validation commands
- [x] 4.2 Review diff against base worktree and fix regressions
- [x] 4.3 Archive the completed OpenSpec change into `openspec/changes/archive/`
