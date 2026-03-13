## ADDED Requirements

### Requirement: Transcript MUST remain responsive for very long conversations

When a chat session contains a very large number of messages and/or very large message bodies, the transcript area MUST remain usable and scrollable.

The UI MUST NOT mount the full transcript history into the DOM at once. Instead, the UI MUST render only a window around the current viewport (virtualized/windowed rendering) to keep DOM size bounded.

#### Scenario: Open a long session without freezing
- **GIVEN** a chat session contains a very long transcript history
- **WHEN** user opens that session in the Chat page
- **THEN** the transcript area remains scrollable and interactive
- **AND** the UI does not attempt to render the entire history into the DOM at once

### Requirement: Transcript MUST support loading older messages by scrolling up

When older messages exist beyond what is currently loaded, the transcript MUST allow loading more history by scrolling upward.

When the user reaches the top of the transcript, the UI MUST request older messages and prepend them to the transcript without losing the user's reading position.

#### Scenario: Scroll to top loads earlier history
- **GIVEN** the transcript currently shows only the most recent portion of a session
- **WHEN** user scrolls to the top of the transcript
- **THEN** the UI loads older messages and prepends them above the current content
- **AND** the user's viewport does not jump away from their current reading position

### Requirement: Auto-follow MUST respect user scrolling

When the user is at the bottom of the transcript, the UI MUST follow new streaming output and newly appended messages.

When the user scrolls up (not at bottom), the UI MUST NOT force-scroll back to bottom during new message arrival, and it MUST provide a clear action to jump back to the latest message.

#### Scenario: User scrolls up during streaming
- **GIVEN** assistant output is streaming into the current session
- **WHEN** user scrolls up away from the bottom
- **THEN** the transcript does not force-scroll back to bottom
- **AND** the UI offers an explicit way to jump back to the latest message

