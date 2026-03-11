## ADDED Requirements

### Requirement: Repo Library MUST extract cards and evidence without Python runtime
The system MUST extract knowledge cards and evidence using Go runtime components.

The system MUST NOT require a Python runtime on end-user machines for card extraction.

#### Scenario: Extraction runs after validated report is persisted
- **WHEN** a validated formal report is written to the snapshot report path
- **THEN** the backend extracts cards and evidence using Go components
- **AND** persists cards and evidence linked to the snapshot and analysis run

### Requirement: Evidence extraction MUST deduplicate file references
The extractor MUST deduplicate evidence items per card using a stable key based on path, line, dimension, and claim text.

#### Scenario: Duplicate evidence references are collapsed
- **WHEN** the report and subagent results contain overlapping evidence references
- **THEN** the extractor stores only one evidence item per deduplication key
- **AND** retains the most informative snippet when available

