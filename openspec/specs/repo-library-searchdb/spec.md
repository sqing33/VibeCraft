# repo-library-searchdb Specification

## Purpose
TBD - created by archiving change github-kb-searchdb. Update Purpose after archive.
## Requirements
### Requirement: Repo Library MUST treat search.db as a rebuildable derived index
The system MUST store search-derived artifacts (chunks, FTS index, vectors, row maps, metadata) in a dedicated SQLite database `search.db`.

The system MUST NOT store any business-unique truth only in `search.db`; all `search.db` contents MUST be rebuildable from `state.db` plus snapshot artifacts (`report.md`, cards/evidence).

#### Scenario: search.db corruption does not break analysis truth
- **WHEN** `search.db` is missing or corrupted
- **THEN** the system still serves repository detail, snapshots, reports, cards, and evidence from `state.db`
- **AND** the system can rebuild `search.db` without re-running the repository analysis

### Requirement: Repo Library MUST version and validate search.db metadata
The system MUST store metadata in `search.db` including schema version, embedding model identifier, embedding dimension, chunk strategy version, and scoring version.

The system MUST detect incompatible versions at runtime and require an index rebuild before serving vector search results.

#### Scenario: Chunk strategy changes require rebuild
- **WHEN** the application detects that `chunk_strategy_version` differs from the running code version
- **THEN** the system marks the index as stale
- **AND** a rebuild operation is required before vector results are returned

### Requirement: Repo Library MUST generate stable chunk identifiers
The system MUST generate stable `chunk_id` values for indexed chunks:
- `card:{card_id}`
- `evidence:{evidence_id}`
- `report:{snapshot_id}:{heading_path_hash}`

The system MUST compute `content_hash = sha256(search_text)` for each chunk to support incremental refresh.

#### Scenario: Rebuild produces identical chunk ids
- **WHEN** the same snapshot is indexed twice with identical inputs
- **THEN** the generated `chunk_id` values remain identical
- **AND** unchanged chunks are not duplicated in the index

### Requirement: Repo Library MUST separate display_text and search_text for chunks
The system MUST store `display_text` for UI rendering and `search_text` for FTS/embedding.

The system MUST allow `search_text` to include flattened tags, symbol refs, and evidence refs without polluting `display_text`.

#### Scenario: Search optimization does not alter user-visible text
- **WHEN** tags or symbol references are added to improve retrieval
- **THEN** the system updates `search_text`
- **AND** preserves the original `display_text` used by the UI

### Requirement: Repo Library MUST provide rebuild operations for search.db
The system MUST support rebuild operations:
- rebuild all repositories
- rebuild a specific snapshot
- rebuild a specific analysis run (mapped to its snapshot)

#### Scenario: Rebuild snapshot after embedding resources are added
- **WHEN** embedding resources become available after a previous indexing attempt
- **THEN** the system can rebuild the affected snapshot index
- **AND** vector search becomes available without re-running analysis

