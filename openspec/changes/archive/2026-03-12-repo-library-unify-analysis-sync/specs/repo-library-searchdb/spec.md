## MODIFIED Requirements

### Requirement: Repo Library MUST treat search.db as a rebuildable derived index
The system MUST store search-derived artifacts (chunks, FTS index, vectors, row maps, metadata) in a dedicated SQLite database `search.db`.

The system MUST NOT store any business-unique truth only in `search.db`; all `search.db` contents MUST be rebuildable from `state.db` plus analysis artifacts (`report.md`, cards/evidence).

#### Scenario: search.db corruption does not break analysis truth
- **WHEN** `search.db` is missing or corrupted
- **THEN** the system still serves repository detail, analyses, reports, cards, and evidence from `state.db`
- **AND** the system can rebuild `search.db` without re-running the repository analysis

### Requirement: Repo Library MUST generate stable chunk identifiers
The system MUST generate stable `chunk_id` values for indexed chunks:
- `card:{card_id}`
- `evidence:{evidence_id}`
- `report:{analysis_result_id}:{heading_path_hash}`

The system MUST compute `content_hash = sha256(search_text)` for each chunk to support incremental refresh.

#### Scenario: Rebuild produces identical chunk ids
- **WHEN** the same analysis result is indexed twice with identical inputs
- **THEN** the generated `chunk_id` values remain identical
- **AND** unchanged chunks are not duplicated in the index

### Requirement: Repo Library MUST provide rebuild operations for search.db
The system MUST support rebuild operations:
- rebuild all repositories
- rebuild a specific analysis result

#### Scenario: Rebuild analysis result after embedding resources are added
- **WHEN** embedding resources become available after a previous indexing attempt
- **THEN** the system can rebuild the affected analysis result index
- **AND** vector search becomes available without re-running analysis

