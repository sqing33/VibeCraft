## ADDED Requirements

### Requirement: Formal report validation MUST classify issues as fatal or warning
The validator MUST classify validation findings into blocking fatal errors and non-blocking warnings.

Fatal errors MUST prevent publishing the report as the official snapshot report.

Warnings MUST be recorded but MUST NOT block publishing the report when no fatal errors exist.

#### Scenario: Warning does not block finalization
- **WHEN** a candidate report has warnings but no fatal errors
- **THEN** the system finalizes the report as the official snapshot report
- **AND** records the warnings in analysis run diagnostics

#### Scenario: Fatal error blocks finalization
- **WHEN** a candidate report contains one or more fatal validation errors
- **THEN** the system does not publish it as the official snapshot report
- **AND** downstream card extraction and index refresh do not run from that draft

