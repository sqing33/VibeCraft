## ADDED Requirements

### Requirement: Repo Library detail MUST provide a reanalyze entrypoint that reuses analysis parameters
The repository detail page MUST expose an action that reuses the currently selected analysis parameters and navigates to the analysis submission form, enabling users to run a comparison analysis with a different CLI tool or model.

#### Scenario: User triggers reanalyze from repository detail
- **WHEN** the user clicks the reanalyze action on a repository detail page
- **THEN** the UI navigates to the Analyze Repo form
- **AND** pre-fills the form with the selected analysis request parameters

### Requirement: Repo Library repository list sidebar SHOULD provide a quick prefill action
The repository list sidebar SHOULD provide a quick action to prefill the Analyze Repo form using the latest available analysis for that repository.

#### Scenario: User triggers quick prefill from repository list
- **WHEN** the user clicks the quick prefill action on a repository item
- **THEN** the UI pre-fills the Analyze Repo form with the latest analysis parameters available for that repository

