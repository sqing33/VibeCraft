## MODIFIED Requirements

### Requirement: Expert settings can be saved independently from LLM settings
The system MUST provide `PUT /api/v1/settings/experts` so the UI can create, update, enable, disable, and delete user-managed experts without editing the models tab.

Expert configs MAY additionally store builder provenance fields, including `builder_session_id` and `builder_snapshot_id`, so a published expert can be traced back to its generation history.

#### Scenario: Save published expert with builder provenance
- **WHEN** client publishes a builder snapshot into an expert
- **THEN** the saved expert config includes the source builder session id and snapshot id
- **AND** later reads of expert settings expose those references to the UI
