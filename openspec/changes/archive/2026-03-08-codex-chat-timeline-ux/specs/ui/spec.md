## ADDED Requirements

### Requirement: Chat process timeline MUST preserve runtime order and collapse heavy command output
When the chat page renders Codex runtime activity, it MUST present thinking, tool, plan, question, progress, system, and answer entries in chronological timeline order.

The UI MUST NOT merge all thinking into one visual block when those thinking events are separated by other runtime activity.

Tool entries MUST show the executed command immediately, but their `stdout/stderr` content MUST remain collapsed by default until the user explicitly expands that entry. This default-collapsed behavior MUST also apply to failed commands.

#### Scenario: Thinking, tool, and thinking render as separate timeline cards
- **WHEN** one turn contains `thinking → tool → thinking`
- **THEN** the UI renders two separate thinking cards with the tool card between them
- **AND** the answer card keeps its own highlighted style without being forcibly moved ahead of the timeline

#### Scenario: Tool output is collapsed by default
- **WHEN** a tool entry contains captured `stdout` or `stderr`
- **THEN** the timeline initially shows only the command summary and output metadata
- **AND** the raw output is shown only after the user clicks to expand that tool entry
