## ADDED Requirements

### Requirement: Chat page MUST use an immersive workspace layout
The chat page MUST present an immersive workspace layout composed of a narrow session sidebar and a main conversation canvas.

The session sidebar MUST remain visually subordinate to the conversation canvas and MUST primarily contain session navigation plus lightweight session actions.

The conversation canvas MUST prioritize message reading and writing over secondary controls.

#### Scenario: User opens chat page
- **WHEN** user navigates to `#/chat`
- **THEN** the page shows a narrow session sidebar on the left
- **AND** a larger conversation canvas on the right
- **AND** the conversation canvas is the dominant visual area

### Requirement: Conversation canvas MUST constrain readable content width
Within the conversation canvas, the visible transcript content MUST use a constrained readable width rather than stretching message content across the full available area.

This constrained width MUST still allow long-form responses to remain readable while preserving a large amount of surrounding whitespace.

#### Scenario: User reads a long assistant response
- **WHEN** the conversation canvas has enough horizontal space for a wide layout
- **THEN** message content remains centered in a readable-width column
- **AND** the content does not stretch edge-to-edge across the full canvas

### Requirement: Composer MUST remain anchored to the bottom of the conversation canvas
The chat composer MUST remain anchored to the bottom of the conversation canvas so that message scrolling does not move the input area out of place.

The transcript area MUST scroll independently from the composer area.

#### Scenario: Transcript grows during a long conversation
- **WHEN** the active conversation contains enough messages to overflow vertically
- **THEN** the transcript area becomes scrollable
- **AND** the composer remains visible at the bottom of the canvas
