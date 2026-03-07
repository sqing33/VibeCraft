## MODIFIED Requirements

### Requirement: Repo Library MUST extract knowledge cards from analyzer outputs
The system MUST derive structured knowledge cards from analyzer outputs including `report.md` and, when available, `subagent_results.json`.

Knowledge cards MUST support at least `project_characteristic`, `feature_pattern`, `risk_note`, and `integration_note` types.

The extractor MUST tolerate minor heading-label drift in AI-generated markdown reports as long as the report still follows the overall structured analysis format.

#### Scenario: Successful card extraction
- **WHEN** an analysis run finishes with a generated report
- **THEN** the system extracts one or more knowledge cards from the report
- **AND** persists the cards linked to the snapshot and analysis run

#### Scenario: AI report headings drift slightly
- **WHEN** an AI-generated report uses semantically equivalent headings or labels with minor wording variation
- **THEN** the extractor still maps those sections into cards and evidence when enough structure remains
- **AND** the system does not fail solely because one heading text changed slightly
