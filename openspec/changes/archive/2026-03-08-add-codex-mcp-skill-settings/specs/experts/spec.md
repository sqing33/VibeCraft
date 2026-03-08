## ADDED Requirements

### Requirement: Expert enabled_skills MUST constrain runtime skill injection
When an expert declares `enabled_skills`, the system MUST treat that list as a runtime constraint for CLI chat sessions that support skill guidance.

If an expert does not declare `enabled_skills`, the runtime MAY use the full tool-enabled skill set.
If an expert declares `enabled_skills`, the runtime MUST use only the intersection of tool-enabled skills and the expert list.

#### Scenario: Expert without enabled_skills uses tool defaults
- **WHEN** a chat session runs with an expert that has no `enabled_skills`
- **THEN** the runtime uses the full skill set enabled for the selected CLI tool

#### Scenario: Expert enabled_skills narrows runtime set
- **WHEN** a chat session runs with an expert whose `enabled_skills` contains `ui-ux-pro-max`
- **AND** the selected CLI tool has `ui-ux-pro-max` and `worktree-lite` enabled
- **THEN** the runtime injects only `ui-ux-pro-max`
