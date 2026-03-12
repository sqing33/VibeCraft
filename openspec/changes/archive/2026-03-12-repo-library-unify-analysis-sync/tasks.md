## 1. Schema And Store

- [x] 1.1 Add `repo_analysis_results` table and migrate existing `repo_snapshots` + `repo_analysis_runs` into it (transactional table-rebuild migration)
- [x] 1.2 Replace `repo_knowledge_cards` snapshot/run linkage with a single `analysis_result_id` and migrate existing card rows
- [x] 1.3 Remove snapshot/run queries and structs from store layer; introduce `RepoAnalysisResult` APIs for create/list/get/finalize/chat-link
- [x] 1.4 Update go-searchdb schema to key chunks by `analysis_result_id` (derived DB rebuildable) and remove snapshot/run fields

## 2. Backend Repo Library Service And API

- [x] 2.1 Update Repo Library service pipeline layout to be analysis-result-centric (`analyses/<analysis_id>`), while respecting persisted `storage_path` for historical data
- [x] 2.2 Update ingestion endpoint and repository detail payloads to expose analysis results (single selector) and remove snapshot/run DTOs
- [x] 2.3 Update report/cards/evidence APIs to filter by `analysis_result_id` and keep UI-critical fields (generated time, stack summary, etc.)

## 3. Sync Latest Reply + Report Validation

- [x] 3.1 Refactor report validation to support “validate latest reply directly”; persist if valid
- [x] 3.2 Implement short format-repair prompt (validator errors + required structure) and use it for both initial final-report retries and sync retries (no re-sending the initial large prompt)
- [x] 3.3 Add/update tests for validator retry prompts and sync behavior

## 4. UI Updates

- [x] 4.1 Replace snapshot/run selectors with a single analysis selector on repository detail page; adjust data loading and derived panels
- [x] 4.2 Update pattern search result payload handling and “打开仓库详情” navigation to use analysis-result identifiers
- [x] 4.3 Verify Repo Library routes scroll correctly and controls render without truncation/regressions

## 5. Cleanup And Verification

- [x] 5.1 Remove unused snapshot/run code paths, dead API routes, and stale types
- [x] 5.2 Run backend/unit tests and UI build; manually verify multi-analysis selection and sync behavior in the browser
