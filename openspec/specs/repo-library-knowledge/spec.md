# repo-library-knowledge Specification

## Purpose
TBD - created by archiving change add-repo-library. Update Purpose after archive.
## Requirements
### Requirement: Repo Library MUST extract knowledge cards from analyzer outputs
The system MUST derive structured knowledge cards from analyzer outputs including `report.md` and, when available, `subagent_results.json`.

Knowledge cards MUST support at least `project_characteristic`, `feature_pattern`, `risk_note`, and `integration_note` types.

The extractor MUST tolerate minor heading-label drift in AI-generated markdown reports as long as the report still follows the overall structured analysis format.

#### Scenario: Successful card extraction
- **WHEN** an analysis result finishes with a generated report
- **THEN** the system extracts one or more knowledge cards from the report
- **AND** persists the cards linked to the analysis result

#### Scenario: AI report headings drift slightly
- **WHEN** an AI-generated report uses semantically equivalent headings or labels with minor wording variation
- **THEN** the extractor still maps those sections into cards and evidence when enough structure remains
- **AND** the system does not fail solely because one heading text changed slightly

### Requirement: Repo Library MUST preserve evidence for each card
Each knowledge card MUST store one or more evidence items when supporting evidence is available.

Each evidence item MUST include at least source path, source line, and a dimension or classification label when such data is available from extraction.

#### Scenario: Card contains evidence references
- **WHEN** a card is extracted from report sections that reference concrete files
- **THEN** the card stores evidence entries with file path and line information
- **AND** the UI can query these evidence entries independently

### Requirement: Repo Library MUST expose repository and card detail data
The system MUST provide APIs to retrieve repository detail, analyses, cards, and card evidence for UI presentation.

#### Scenario: User opens repository detail
- **WHEN** the UI requests repository detail for an analyzed repository
- **THEN** the backend returns repository metadata, recent analyses, and linked cards
- **AND** the UI can later request evidence for an individual card

