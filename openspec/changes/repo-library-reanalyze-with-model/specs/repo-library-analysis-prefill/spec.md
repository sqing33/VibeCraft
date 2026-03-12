## ADDED Requirements

### Requirement: Repo Library UI MUST support one-shot analysis request prefill
The system MUST allow the UI to carry a one-shot analysis request draft across Repo Library pages and prefill the analysis submission form.

The draft MUST include at least: `repo_url`, `ref`, `features`, `depth`, `language`, `analyzer_mode`, and optional `cli_tool_id/model_id`.

After the draft has been applied to the analysis submission form, the system MUST clear the draft.

#### Scenario: Prefill draft is applied and cleared
- **WHEN** the user navigates to the Analyze Repo form with a prefill draft present
- **THEN** the UI pre-populates form fields from that draft
- **AND** clears the draft immediately after applying it

### Requirement: Repo Library UI MUST allow reanalyze entrypoints to populate the prefill draft
The system MUST allow Repo Library pages to set the prefill draft based on an existing analysis result.

#### Scenario: User reuses an existing analysis to prefill a new request
- **WHEN** the user triggers a reanalyze action from an existing repository analysis result
- **THEN** the UI creates a prefill draft that mirrors the existing analysis request parameters
- **AND** navigates the user to the Analyze Repo form

