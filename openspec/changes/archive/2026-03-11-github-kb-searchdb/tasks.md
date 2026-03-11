## 1. Repo Preparation (Go)

- [x] 1.1 Implement GitHub ZIP fetch + unzip with git clone fallback into Repo Library snapshot source dir
- [x] 1.2 Generate minimal `code_index.json` for prepared snapshot and persist resolved_ref/commit_sha when available
- [x] 1.3 Enforce snapshot immutability (never overwrite an existing snapshot source; always create new snapshot id)

## 2. Formal Report Validation (Go)

- [x] 2.1 Port formal report validator to Go (structure + feature mapping + evidence ref format + extractability prechecks)
- [x] 2.2 Add fatal vs warning classification and wire retry loop to only trigger on fatal errors

## 3. Knowledge Extraction (Go)

- [x] 3.1 Port cards/evidence extraction to Go (consume `report.md` and optional `subagent_results.json`)
- [x] 3.2 Implement evidence deduplication rule and persist via existing `ReplaceRepoKnowledge`

## 4. Search Index (search.db)

- [x] 4.1 Add `search.db` schema + metadata table + stable chunk_id/content_hash rules
- [x] 4.2 Implement FTS5 indexing and keyword search over `report_section/card/evidence` chunks
- [x] 4.3 Implement sqlite-vec integration (load extension, vec0 table, KNN query) with safe fallback to FTS-only
- [x] 4.4 Implement fusion scoring with configurable weights and card-first preference
- [x] 4.5 Implement rebuild operations: rebuild all / snapshot / run

## 5. Repo Library Wiring (Replace Python)

- [x] 5.1 Replace `prepareRepositoryForAI` Python call with Go prepare pipeline
- [x] 5.2 Replace `validateCandidateReport` Python call with Go validator
- [x] 5.3 Replace `postProcessAIReport` Python calls with Go extraction + Go search index refresh
- [x] 5.4 Replace `RepoLibrary.Search` implementation to use Go search index (record search history as before)

## 6. API & UI Compatibility

- [x] 6.1 Keep `POST /api/v1/repo-library/search` compatible; add optional fields (source_kinds/refresh/min_score) without breaking callers
- [x] 6.2 Add snapshot file read endpoint safeguards for file:line navigation (repo-relative within snapshot root)

## 7. Tests & Docs

- [x] 7.1 Add unit tests for chunk id stability and evidence dedupe behavior
- [x] 7.2 Add integration test for indexing + search fallback behavior when vec/embedder unavailable
- [x] 7.3 Update `PROJECT_STRUCTURE.md` to include new search/index modules and removal of Python runtime dependency
