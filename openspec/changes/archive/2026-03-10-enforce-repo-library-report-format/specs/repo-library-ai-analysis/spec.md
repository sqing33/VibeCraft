## MODIFIED Requirements

### Requirement: Automated repository analysis MUST produce a final markdown report compatible with Repo Library post-processing
The final automated AI turn MUST produce the final analysis report as markdown in the expected Repo Library report structure so that downstream card extraction and search indexing can continue to operate.

Only the automated final-report turn MUST enforce the strict report template. Later user-driven follow-up turns in the linked chat session MUST remain free-form unless the user explicitly chooses to sync the latest reply back into Repo Library.

The system MUST validate the candidate final report before treating it as the official snapshot report, and MUST retry report generation when the validator returns blocking errors.

If validation cannot pass within the retry budget, the system MUST preserve the last invalid draft as diagnostic output and MUST NOT finalize Repo Library cards or search data from that invalid draft.

#### Scenario: Analysis chat produces final report markdown
- **WHEN** the automated analysis reaches its final report step
- **THEN** the assistant output is persisted as the snapshot report markdown
- **AND** the downstream card extraction step consumes that report without manual translation or editing

#### Scenario: Follow-up chat reply does not overwrite report automatically
- **WHEN** the user continues the linked analysis chat after the automated report has completed
- **THEN** the new assistant reply does not automatically replace the stored report
- **AND** Repo Library state changes only when the explicit sync action is invoked

#### Scenario: Blocking validation error delays report finalization
- **WHEN** the automated final-report turn returns markdown that fails Repo Library validation
- **THEN** the system keeps that version as a candidate draft instead of the official report
- **AND** the system requests a corrected full report before finalizing the analysis

#### Scenario: Validation failure prevents finalization
- **WHEN** all allowed correction attempts still fail validation
- **THEN** the analysis result is not finalized from the invalid markdown
- **AND** Repo Library keeps diagnostic artifacts for inspection instead of publishing empty derived results
