# repo-library-report-validation Specification

## Purpose
TBD - created by archiving change enforce-repo-library-report-format. Update Purpose after archive.
## Requirements
### Requirement: Formal Repo Library reports MUST pass machine validation before becoming official results
The system MUST treat the AI-produced final report as a candidate artifact until it passes machine validation.

Machine validation MUST check required report structure, feature-to-section mapping, evidence reference format, and whether the report can yield at least one knowledge card through Repo Library extraction.

#### Scenario: Candidate report passes validation
- **WHEN** an automated repository analysis produces a candidate final report
- **THEN** the system validates the report before finalizing analysis results
- **AND** only a passing report is persisted as the official snapshot report
- **AND** downstream card extraction and search refresh continue from that validated report

#### Scenario: Candidate report fails validation
- **WHEN** a candidate final report does not satisfy structure or extractability checks
- **THEN** the system does not treat it as the official snapshot report
- **AND** the system records validation errors for follow-up handling

### Requirement: Repo Library MUST retry formal report generation with validator feedback
When a candidate report fails validation, the system MUST request a full corrected report from the linked AI chat session using the validation failures as explicit feedback.

The retry loop MUST stop after a bounded number of attempts.

#### Scenario: Retry after validation failure
- **WHEN** the validator returns one or more blocking errors for a candidate report
- **THEN** the system sends a corrective follow-up prompt to the same analysis chat session
- **AND** the prompt includes the blocking validation errors
- **AND** the assistant is asked to return a full corrected final report instead of a patch or explanation

#### Scenario: Validation eventually succeeds within retry budget
- **WHEN** one of the retry attempts produces a passing report
- **THEN** the system finalizes the analysis from that passing version
- **AND** earlier failed attempts remain diagnostic artifacts only

### Requirement: Repo Library MUST preserve invalid report artifacts when retries are exhausted
If all retry attempts are exhausted without a passing report, the system MUST preserve the final invalid report draft together with its validation result for diagnosis.

The system MUST NOT publish cards or search index updates from an invalid final draft.

#### Scenario: Retries exhausted without a passing report
- **WHEN** the final retry attempt still fails validation
- **THEN** the system stores the invalid final draft and validation output as diagnostic artifacts
- **AND** the analysis does not update official cards or search corpus from that invalid draft
