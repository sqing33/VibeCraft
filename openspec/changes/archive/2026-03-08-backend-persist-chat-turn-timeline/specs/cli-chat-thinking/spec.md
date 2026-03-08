## ADDED Requirements

### Requirement: Persisted Codex reasoning MUST avoid duplicate summary and raw text
For one Codex reasoning item, the system MUST prefer readable `summaryTextDelta` content for the visible thinking timeline whenever it is available.

If readable summary content has already been emitted for that reasoning item, later raw `textDelta` content for the same item MUST NOT be appended to the visible persisted timeline entry.

If no readable summary content exists for that reasoning item, the system MAY use raw reasoning text as the visible fallback.

#### Scenario: Summary reasoning suppresses raw reasoning duplication
- **WHEN** one Codex reasoning item emits both `item/reasoning/summaryTextDelta` and `item/reasoning/textDelta`
- **THEN** the visible persisted thinking timeline keeps the readable summary content
- **AND** the raw reasoning text does not create duplicate visible characters in the same turn timeline

#### Scenario: Raw reasoning is used when no summary exists
- **WHEN** a Codex reasoning item emits raw `item/reasoning/textDelta` content but no readable summary deltas
- **THEN** the system persists that raw reasoning as the visible thinking content for the timeline
