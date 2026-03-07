# repo-library-knowledge Specification

## Purpose
TBD - created by archiving change add-repo-library. Update Purpose after archive.
## Requirements
### Requirement: Repo Library MUST extract knowledge cards from analyzer outputs
The system MUST derive structured knowledge cards from analyzer outputs including `report.md` and, when available, `subagent_results.json`.

Knowledge cards MUST support at least `project_characteristic`, `feature_pattern`, `risk_note`, and `integration_note` types.

#### Scenario: Successful card extraction
- **WHEN** an analysis run finishes with a generated report
- **THEN** the system extracts one or more knowledge cards from the report
- **AND** persists the cards linked to the snapshot and analysis run

#### Scenario: Subagent results are unavailable
- **WHEN** an analysis run does not produce `subagent_results.json`
- **THEN** the system still extracts cards from `report.md`
- **AND** marks any lower-confidence conclusions accordingly

### Requirement: Repo Library MUST preserve evidence for each card
Each knowledge card MUST store one or more evidence items when supporting evidence is available.

Each evidence item MUST include at least source path, source line, and a dimension or classification label when such data is available from extraction.

#### Scenario: Card contains evidence references
- **WHEN** a card is extracted from report sections that reference concrete files
- **THEN** the card stores evidence entries with file path and line information
- **AND** the UI can query these evidence entries independently

### Requirement: Repo Library MUST expose repository and card detail data
The system MUST provide APIs to retrieve repository detail, snapshots, cards, and card evidence for UI presentation.

#### Scenario: User opens repository detail
- **WHEN** the UI requests repository detail for an analyzed repository
- **THEN** the backend returns repository metadata, recent snapshots, and linked cards
- **AND** the UI can later request evidence for an individual card

