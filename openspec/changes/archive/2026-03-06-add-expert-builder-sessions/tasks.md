## 1. Spec and persistence model

- [x] 1.1 Add builder session/message/snapshot schema and migrate SQLite
- [x] 1.2 Add store CRUD for builder sessions, messages, snapshots, and publish metadata
- [x] 1.3 Add tests for migration and store operations

## 2. Backend expert-builder session API

- [x] 2.1 Add create/list/detail/message/publish endpoints for builder sessions
- [x] 2.2 Refactor expert builder service to create snapshots from session history
- [x] 2.3 Add API tests covering long conversation and publish flow

## 3. Expert config provenance

- [x] 3.1 Extend expert config/settings payload with builder session and snapshot references
- [x] 3.2 Make publish write source provenance and support continue-optimization lookup

## 4. Frontend expert workbench

- [x] 4.1 Refactor ExpertSettingsTab into session-driven long conversation modal
- [x] 4.2 Show session history and snapshot list, and allow selecting a snapshot preview
- [x] 4.3 Support continue refining a published expert from saved history

## 5. Validation and docs

- [x] 5.1 Update PROJECT_STRUCTURE.md and related docs/specs
- [x] 5.2 Run backend tests and frontend build
- [x] 5.3 Archive the change after implementation
