# chat-page-immersive-layout Specification

## Purpose

Define the immersive chat workspace layout for `#/chat`, including a narrow session rail, a dominant conversation canvas, constrained readable transcript width, and a bottom-anchored composer.
## Requirements
### Requirement: Chat page MUST use an immersive workspace layout
The chat page MUST render inside the shared workspace app shell.

Within that shell, the top of the left rail MUST show the global product navigation, the middle of the left rail MUST show chat session browsing and creation, and the bottom of the left rail MUST show shared status and utility controls.

The right conversation canvas MUST remain the visually dominant bordered panel on the right.

#### Scenario: User opens chat page
- **WHEN** user navigates to `#/chat`
- **THEN** the shared workspace shell is visible
- **AND** the chat lane is visually highlighted in the global navigation
- **AND** the middle left sidebar region shows the chat session list and new-session action
- **AND** the conversation canvas remains the dominant bordered panel on the right

#### Scenario: User switches back to chat from another product lane
- **WHEN** the user navigates from Orchestrations or Github 知识库 back to `#/chat`
- **THEN** the shared workspace shell remains mounted
- **AND** the middle left sidebar region swaps back to the chat session list
- **AND** the right content region swaps back to the conversation workspace

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

### Requirement: Conversation header MUST use a three-zone layout
The conversation canvas header MUST place session metadata on the left, the active conversation title in the visual center, and status plus conversation actions on the right.

The centered title MUST remain visually centered when the left and right zones have different content widths.

Long metadata or title text MUST truncate instead of forcing the header to grow vertically in the default desktop layout.

#### Scenario: Active session is displayed
- **WHEN** an active session is open in the conversation canvas
- **THEN** the header shows session identifier and model metadata on the left
- **AND** the conversation title is displayed in the center of the header
- **AND** the status chip and fork action are displayed on the right

#### Scenario: No active session is selected
- **WHEN** no chat session is currently active
- **THEN** the header still preserves the same three-zone layout
- **AND** the center area shows the empty-state title text

