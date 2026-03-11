## ADDED Requirements

### Requirement: Repo Library search MUST fuse FTS and vector retrieval when available
The search implementation MUST combine keyword retrieval (FTS) and vector retrieval (sqlite-vec) into a single ranked result set when vector retrieval is available.

If vector retrieval is unavailable (missing embedder resources or sqlite-vec extension), the system MUST degrade to keyword-only search and record diagnostics.

#### Scenario: Vector unavailable falls back to keyword search
- **WHEN** sqlite-vec cannot be loaded or embedding resources are missing
- **THEN** the system still returns keyword search results when possible
- **AND** includes diagnostics indicating vector retrieval is disabled

### Requirement: Search results SHOULD prioritize cards as primary answers
Search results MUST be traceable to repository, snapshot, and source kind, and SHOULD prioritize `card` results as primary answer units when multiple source kinds match.

#### Scenario: Card is returned with supporting evidence
- **WHEN** the query matches both cards and evidence chunks
- **THEN** the system returns a card-level result in the top results when available
- **AND** includes a small set of supporting evidence references for inspection

