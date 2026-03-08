# 前端 UI

## Purpose

React SPA 提供 Kanban 列表页 + Workflow 详情页（DAG + Terminal Pool）交互，通过 HTTP/WebSocket 与 daemon 通信。
## Requirements
### Requirement: Technology Stack

The system MUST use React + TypeScript + Vite for the frontend build. The system MUST use Tailwind CSS + HeroUI (`@heroui/react`) for styling and UI components. The system MUST use Zustand for state management. The system MUST use React Flow + dagre for DAG visualization. The system MUST use xterm.js with fit addon for terminal rendering.

#### Scenario: Frontend builds successfully

- **WHEN** running `npm run build` in the ui/ directory
- **THEN** the build completes without errors
- **AND** produces static assets in ui/dist/

### Production hides demo-only tools by default

In production builds (`import.meta.env.DEV === false`), the UI MUST hide demo-only tools (e.g. oneshot demo execution) by default. In development builds, demo/diagnostic tools MAY be visible to aid debugging.

#### Scenario: Demo tools hidden in production

- **WHEN** the UI is served from `ui/dist/` by the daemon
- **THEN** demo-only actions are not visible by default

#### Scenario: Demo tools visible in development

- **WHEN** the UI is running under Vite dev server (`import.meta.env.DEV === true`)
- **THEN** demo/diagnostic actions MAY be visible

### Workflows-first information architecture

The UI MUST provide a primary navigation centered around Workflows. Connection settings and diagnostics MUST be accessible but MUST NOT dominate the default workflow-oriented UI.

#### Scenario: Opening the app lands on Workflows

- **WHEN** user opens the application
- **THEN** the Workflows Kanban is the primary visible content
- **AND** diagnostics/settings are reachable but not the main focus

### Settings contains connection and diagnostics

The UI MUST provide a Settings entry (e.g. drawer/modal) that contains daemon URL switching and diagnostics (version/paths/experts). Changing daemon URL MUST persist to local storage and MUST trigger a health check.

#### Scenario: Change daemon URL in Settings

- **WHEN** user updates the daemon URL in Settings and applies the change
- **THEN** the new URL is saved to local storage
- **AND** the UI performs `GET /api/v1/health` against the new URL
- **AND** the UI updates connection status (Health/WS)

### Kanban List Page

The system MUST render a Kanban board at route `/` with four columns: Todo, Running, Done, Failed. Each card MUST display: title, workspace, last updated time, run mode (Auto/Manual), running node count. The system MUST provide a `+ New` button to create workflows with fields: title, workspace, mode, master expert.

#### Scenario: Display workflow cards

- **WHEN** user opens the application
- **AND** health check passes
- **THEN** workflows are displayed in Kanban columns grouped by status

#### Scenario: Create new workflow

- **WHEN** user clicks `+ New` and fills in title, workspace, mode
- **THEN** a new workflow is created via POST /api/v1/workflows
- **AND** the card appears in the Todo column

### Detail Page Layout

The system MUST render workflow details at route `#/workflows/:id` with a left-right split layout. The top MUST include a run control bar with execution breakpoint toggle (auto ↔ manual) and `Approve all runnable` button (visible only in manual mode). The left pane MUST show DAG view. The right pane MUST show Terminal Pool.

#### Scenario: View running workflow details

- **WHEN** user clicks a running workflow
- **THEN** the detail page shows DAG on the left and terminal outputs on the right

### DAG View

The system MUST use React Flow with dagre auto-layout to render nodes and edges. Nodes MUST display title, expert, and status with color coding (green/yellow/red/gray). Clicking a node MUST scroll the right terminal to the corresponding pane and highlight it. Error nodes MUST show a red breathing animation (CSS).

#### Scenario: Node click triggers terminal focus

- **WHEN** user clicks a node in the DAG view
- **THEN** the right terminal scrolls to the corresponding pane
- **AND** the pane is visually highlighted

### Terminal Pool

The system MUST render terminal panes for running/selected nodes. Each pane MUST have minimum width `min-w-[420px]` with horizontal scroll container (`overflow-x-auto`). Each pane top bar MUST show node name, status, and Cancel/Retry buttons. Terminals MUST use xterm.js with fit addon, receiving WebSocket log chunks.

#### Scenario: Real-time log rendering

- **WHEN** WebSocket pushes `node.log` events with ANSI content
- **THEN** the corresponding xterm pane renders colored output in real-time

### Manual Mode Interaction

The system MUST allow clicking `Approve all runnable` to start all dependency-satisfied nodes. The system MUST support editing node expert (dropdown) and prompt (textarea). Saving edits MUST call `PATCH /api/v1/nodes/{id}`.

#### Scenario: Edit and approve nodes

- **WHEN** user edits a node's prompt and saves
- **THEN** PATCH /api/v1/nodes/{id} is called
- **AND** clicking Approve starts all runnable nodes

### WebSocket Subscription

The system MUST connect to `GET /api/v1/ws` for real-time event subscription. The system MUST handle events: `workflow.updated`, `dag.generated`, `node.updated`, `execution.started`, `execution.exited`, `node.log`. `node.log` chunks MUST be written directly to the corresponding xterm instance.

#### Scenario: WebSocket reconnection

- **WHEN** WebSocket connection is lost
- **THEN** the client automatically attempts reconnection
- **AND** resumes receiving events after reconnection

### Daemon Communication

The system MUST encapsulate all HTTP API calls in `daemon.ts`. The system MUST support daemon URL configuration via `VITE_DAEMON_URL` environment variable or runtime switching. The system MUST perform health checks via `GET /api/v1/health`.

#### Scenario: Health check on startup

- **WHEN** the application starts
- **THEN** it calls GET /api/v1/health
- **AND** displays connection status to the user

### UI text is Chinese by default

The UI MUST present user-facing copy in Simplified Chinese across the primary workflow path, including navigation labels, action buttons, form labels, status text, empty states, and error prompts.

#### Scenario: User opens workflows page

- **WHEN** user opens the application
- **THEN** primary page titles and action buttons are displayed in Simplified Chinese

#### Scenario: User sees error or empty state

- **WHEN** workflow list or detail page enters error/empty/loading states
- **THEN** the corresponding visible prompts are displayed in Simplified Chinese

### UI supports light and dark theme toggle

The UI MUST provide a user-accessible theme switch for light and dark modes. Theme selection MUST be persisted in local storage and restored on next load.

#### Scenario: User toggles theme

- **WHEN** user switches between light and dark theme in UI settings/topbar
- **THEN** the entire UI theme updates immediately
- **AND** selected theme is saved to local storage

#### Scenario: User reloads app after selecting theme

- **WHEN** user reloads the app after previously selecting dark or light theme
- **THEN** the UI restores the saved theme from local storage

### Default theme is light

When no persisted theme value exists, the UI MUST initialize with light theme.

#### Scenario: First-time visitor

- **WHEN** local storage does not contain a saved theme key
- **THEN** the UI loads in light theme

### Requirement: Settings uses tab navigation and includes LLM configuration

The UI MUST present the existing System Settings as a tabbed view.
The UI MUST provide at least four tabs:

- `基本设置`: contains thinking translation configuration.
- `连接与诊断`: contains daemon URL switching and diagnostics (version/paths).
- `模型`: contains LLM Sources / Model Profiles configuration.
- `专家`: contains expert list, expert details, and AI creation workflow.

#### Scenario: User switches settings tabs

- **WHEN** user opens System Settings
- **THEN** the UI shows multiple tabs including `基本设置`, `连接与诊断`, `模型`, and `专家`
- **AND** switching tabs updates the visible settings content

### Requirement: Basic settings tab can configure thinking translation

The `基本设置` tab MUST provide a `思考过程翻译` settings section.

The section MUST contain exactly these configurable fields:
- `API 源`: selects an existing LLM Source
- `翻译模型`: a manually entered model string
- `需要翻译的 AI 模型`: a multi-select list populated from all configured LLM models

If no LLM Source exists, the UI MUST disable the translation configuration fields and guide the user to configure sources first in the `模型` tab.

If LLM models do not exist yet, the UI MUST disable the target model selector.

Saving the form MUST call `PUT /api/v1/settings/basic` and show success or failure feedback.

#### Scenario: Save thinking translation settings

- **WHEN** user selects a source, enters a translation model, selects one or more target AI models, and clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/basic`
- **AND** the UI shows a success toast on success

#### Scenario: Basic settings disabled before model configuration

- **WHEN** the user opens `基本设置` before configuring any LLM Source
- **THEN** the UI disables the thinking translation fields
- **AND** the UI shows guidance to configure API Sources in the `模型` tab first

### Requirement: UI can edit and save LLM settings

In the `模型` tab, the UI MUST organize LLM settings by API Source.

Each Source card MUST manage:

- Source metadata: `id`, `label`, `base_url`, and source-level SDK provider.
- Source secret: `api_key`.
- A nested model list rendered directly under the API Key field.

The UI MUST NOT present a separate top-level `Models` section.

Each nested model row MUST allow the user to enter one model ID string. The UI MUST use the entered text as the display label and MUST derive the persisted model `id` and `model` by lowercasing the trimmed input.

The UI MUST save changes by calling `PUT /api/v1/settings/llm`.
After saving succeeds, the UI MUST refresh experts by calling `GET /api/v1/experts` so that workflow/node dropdowns can use the latest models.

#### Scenario: User adds a source model inside the source card and saves

- **WHEN** user creates or edits an API Source
- **AND** user adds a model row under that Source card
- **AND** user clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/llm`
- **AND** the UI shows a success toast
- **AND** the UI refreshes the experts list via `GET /api/v1/experts`

#### Scenario: Mixed-case model input is normalized on save

- **WHEN** user enters a mixed-case model ID such as `GPT-5-CODEX` in a Source card
- **AND** user clicks Save
- **THEN** the UI submits lowercase `id` and `model` values such as `gpt-5-codex`
- **AND** the UI keeps the original input as the display label

### Requirement: Model profiles can be tested from the settings UI

In the `模型` settings tab, each nested Source model row MUST provide a `测试` button located to the left of the delete button.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the Source row's current provider/base_url/api_key values and the model row's lowercase-normalized model ID.

The UI MUST show success or failure feedback to the user (e.g. toast).

#### Scenario: User tests a source model row

- **WHEN** user clicks `测试` on a model row with complete Source SDK/API Key/model configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the request uses the Source card's SDK and the model row's lowercase-normalized model ID
- **AND** the UI displays the result to the user

### Requirement: UI SHALL prefer translated reasoning for translated turns

When a translated reasoning stream is available for the current turn, the UI MUST display the translated Chinese reasoning instead of the original reasoning text.

If translation is enabled for the turn but translated text has not arrived yet, the UI SHOULD show a Chinese loading hint rather than immediately rendering the original English reasoning.

If translation fails for the turn, the UI MUST fall back to displaying the original reasoning text.

#### Scenario: Show translated reasoning only

- **WHEN** the current chat turn receives translated reasoning deltas successfully
- **THEN** the reasoning UI displays the translated Chinese text
- **AND** it does not display the original English reasoning at the same time

#### Scenario: Fallback to original reasoning on translation failure

- **WHEN** the current chat turn receives a `chat.turn.thinking.translation.failed` event
- **THEN** the reasoning UI falls back to displaying the original reasoning text for that turn

### Requirement: Expert tab shows expert metadata and status

The `专家` settings tab MUST display each expert's identity and runtime strategy, including at least name, category, description, managed source, primary model, secondary model, fallback summary, enabled skills, and whether the expert is editable.

#### Scenario: Read expert details in settings

- **WHEN** the expert tab loads successfully
- **THEN** the UI renders a list of experts
- **AND** selecting an expert shows its description, model strategy, skill chips, and prompt summary

### Requirement: Expert tab can create experts through AI conversation

The `专家` settings tab MUST provide an `AI 创建专家` entry that opens a conversation-based creation flow.

The creation flow MUST be session-based rather than stateless. It MUST support loading prior messages, continuing the conversation, and previewing historical draft snapshots.

#### Scenario: Generate expert draft in modal

- **WHEN** user opens AI 创建专家 and sends a requirement message
- **THEN** the UI calls the expert generation API
- **AND** shows the assistant reply and a structured expert draft preview side-by-side

#### Scenario: Publish generated expert

- **WHEN** user confirms publish on a valid generated draft
- **THEN** the UI saves the expert through `PUT /api/v1/settings/experts`
- **AND** refreshes both the expert settings payload and `GET /api/v1/experts`

#### Scenario: Continue long conversation

- **WHEN** user sends multiple follow-up messages in the same builder session
- **THEN** the UI appends the full history in order
- **AND** each round updates the latest draft preview instead of replacing the whole session

#### Scenario: Inspect historical draft snapshots

- **WHEN** user opens a builder session with multiple snapshots
- **THEN** the UI shows a snapshot list with version and time
- **AND** selecting a snapshot updates the draft preview

#### Scenario: Continue optimizing an existing expert

- **WHEN** user chooses to optimize a published expert
- **THEN** the UI loads the related builder session if one exists
- **AND** allows continuing the conversation using the saved history

### Requirement: Expert tab supports readonly system experts and editable custom experts

The UI MUST distinguish between builtin / llm-model experts and user-managed experts.

#### Scenario: System expert is readonly

- **WHEN** user selects a builtin or llm-model expert
- **THEN** the UI shows its metadata and readonly badges
- **AND** does not show delete actions for that expert

#### Scenario: Custom expert can be toggled or deleted

- **WHEN** user selects a user-managed expert
- **THEN** the UI allows toggling enabled state and deleting the expert
- **AND** saves the updated custom expert list through the expert settings API

### Requirement: LLM model profiles require a valid Source

Because model rows are edited inside a Source card, each model profile MUST be implicitly bound to exactly one Source.

The UI MUST prevent saving or testing LLM settings when:

- the Source card is missing a valid SDK provider, or
- any nested model row is missing a non-empty model ID.

#### Scenario: Saving is blocked when a nested model ID is missing

- **WHEN** the user clicks Save while any Source card contains an empty model row
- **THEN** the UI shows an error toast describing the missing model ID
- **AND** does not submit the settings update request

#### Scenario: Testing is blocked when Source SDK is missing

- **WHEN** the user clicks `测试` for a model row whose Source card has no SDK provider
- **THEN** the UI shows an error toast describing the missing Source SDK
- **AND** does not submit the test request

### Requirement: UI SHALL provide chat session navigation and page
The UI MUST provide a navigation entry to a dedicated chat page. The chat page MUST support session list browsing and selecting one active session.

The chat page MUST use a two-region layout:
- a fixed-width left session rail for browsing, creating, and managing sessions,
- a dominant right conversation workspace for transcript reading and message composition.

Within the conversation workspace, the UI MUST provide:
- a lightweight header showing the current session context,
- a scrollable transcript region,
- a bottom-anchored composer region that remains stable while the transcript changes.

The visible transcript content SHOULD remain centered within a constrained readable column instead of spanning the full workspace width.

#### Scenario: Open chat page
- **WHEN** user clicks the chat entry in top navigation
- **THEN** the app navigates to `#/chat`
- **AND** the page displays the session rail and conversation workspace

#### Scenario: Switch sessions from the left rail
- **WHEN** user selects another session in the left session rail
- **THEN** the active conversation workspace updates to that session
- **AND** the composer remains in the bottom area of the workspace

#### Scenario: Read long transcript in wide viewport
- **WHEN** the chat page is opened on a desktop-sized viewport
- **THEN** the transcript content appears in a constrained readable column inside the conversation workspace
- **AND** the session rail remains narrower than the conversation workspace

### Requirement: Chat composer MUST support attachment selection and removal
The chat page MUST provide an attachment upload entry in the composer for the active session.

The user MUST be able to select one or more supported files, review the selected attachments before sending, and remove selected attachments before submission.

The composer MUST also support drag-and-drop file upload onto the composer region.

#### Scenario: User selects attachments before sending
- **WHEN** user chooses one or more supported files in the chat composer
- **THEN** the UI shows the selected attachments in the composer before sending

#### Scenario: User removes a selected attachment
- **WHEN** user clicks remove on a selected attachment chip
- **THEN** that attachment is removed from the pending send list

#### Scenario: User drags files into composer
- **WHEN** user drags supported files over the chat composer and drops them
- **THEN** the files are added to the pending attachment list
- **AND** the composer shows a visible drag-active state during the drop interaction

### Requirement: Chat history MUST render message attachment metadata
When a message includes attachments, the chat UI MUST display attachment metadata in the corresponding message bubble.

The initial rendering MUST include at least file name and attachment kind.

For preview-supported attachment types, the UI MUST provide a preview action.

#### Scenario: User message shows sent attachments
- **WHEN** a previously sent message has attachments
- **THEN** the chat history shows those attachments below the message content

#### Scenario: User previews an image attachment
- **WHEN** user clicks preview on an image attachment in chat history
- **THEN** the UI opens an in-app preview modal showing the image

#### Scenario: User previews a PDF attachment
- **WHEN** user clicks preview on a PDF attachment in chat history
- **THEN** the UI opens an in-app preview modal embedding the PDF document

#### Scenario: User previews a markdown attachment
- **WHEN** user clicks preview on a Markdown attachment in chat history
- **THEN** the UI opens an in-app preview modal rendering the Markdown content
- **AND** fenced code blocks inside the Markdown are syntax highlighted

#### Scenario: User previews a code attachment
- **WHEN** user clicks preview on a code or config attachment in chat history
- **THEN** the UI opens an in-app preview modal rendering the file with syntax highlighting and line numbers

#### Scenario: User previews a plain text attachment
- **WHEN** user clicks preview on a plain text attachment in chat history
- **THEN** the UI opens an in-app preview modal showing the text content without syntax highlighting

### Requirement: UI SHALL support multi-turn chat with streaming render
The chat page MUST allow creating sessions, sending user turns, and rendering assistant streaming deltas in real time via WebSocket chat events.

The chat page MUST support sending turns with text, with attachments, or with both.

#### Scenario: Send turn and receive stream
- **WHEN** user sends a message in an active chat session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/turns`
- **AND** assistant response appears incrementally as `chat.turn.delta` events arrive

#### Scenario: Send turn with attachments
- **WHEN** user selects attachments and sends a turn
- **THEN** the UI submits the turn request including the selected attachments
- **AND** the message history refresh shows the attachments on the user message

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

### Requirement: UI SHALL expose compaction and session management actions
The chat page MUST provide actions for manual compaction and session fork. The UI MUST show result feedback for these actions.

#### Scenario: Trigger compact from UI
- **WHEN** user clicks manual compact for a session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/compact`
- **AND** displays success/failure feedback

#### Scenario: Fork session from UI
- **WHEN** user clicks fork on current session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/fork`
- **AND** the new forked session appears in the session list

### Requirement: Settings MUST expose a dedicated CLI tools tab
The UI MUST provide a dedicated `CLI 工具` tab for managing `Codex CLI` and `Claude Code`, including enablement, default model selection, and optional command path override.

#### Scenario: User manages codex and claude tools
- **WHEN** user opens System Settings
- **THEN** the UI shows a `CLI 工具` tab
- **AND** the tab allows managing both primary execution tools

### Requirement: Chat UI SHALL support runtime-first model selection
The chat page MUST let the user select a conversation runtime first and then choose a compatible model from that runtime's model pool.

The runtime list MUST include enabled CLI tools and available SDK providers in the same selector.

At minimum, when corresponding models exist, the selector MUST expose:

- `Codex CLI`
- `Claude Code`
- `OpenAI SDK`
- `Anthropic SDK`

For CLI runtimes, the model selector MUST only show models compatible with that tool's protocol family.

For SDK runtimes, the model selector MUST only show models belonging to the selected provider.

#### Scenario: User chooses codex then openai model
- **WHEN** user selects `Codex CLI` in the chat composer
- **THEN** the model selector only shows OpenAI-compatible models

#### Scenario: User chooses OpenAI SDK
- **WHEN** user selects `OpenAI SDK` in the chat composer
- **THEN** the model selector only shows OpenAI provider models
- **AND** sending the message uses the SDK chat path instead of CLI runtime

#### Scenario: Active SDK session restores selector state
- **WHEN** an active session has `provider="openai"` or `provider="anthropic"` and no `cli_tool_id`
- **THEN** the chat page restores the corresponding SDK runtime option and current model selection

### Requirement: Chat model selectors MUST display the selected model label
The Chat page's tool-first model selectors MUST visibly display the currently selected model label whenever the selected key matches an available model option.

#### Scenario: New-session model selector shows selected label
- **WHEN** user selects a model for a new chat session
- **THEN** the model select control shows that model label in the collapsed field

#### Scenario: Composer model selector shows current session model
- **WHEN** an active session already has a selected tool/model combination
- **THEN** the composer model select shows the matching label instead of an empty field

### Requirement: Chat model selectors MUST visibly render the selected label
The Chat page's model selectors MUST render the selected model label in the collapsed control whenever the selected key matches an available option.

#### Scenario: Composer model select shows selected label
- **WHEN** the current tool/model combination is valid
- **THEN** the composer model select shows the selected model label instead of an empty field

### Requirement: Chat UI MUST render CLI mid-turn feedback
The Chat UI MUST render CLI-generated mid-turn assistant deltas as they arrive, and it MUST render available thinking or progress events without waiting for turn completion.

#### Scenario: Assistant bubble grows during CLI turn
- **WHEN** CLI assistant deltas are received through WebSocket
- **THEN** the pending assistant bubble updates incrementally before completion

### Requirement: Chat UI MUST render structured Codex turn layers
The chat page MUST render Codex runtime activity as layered UI blocks instead of a single mixed pending bubble.

The UI MUST support distinct presentation for progress/system messages, thinking, tool execution, plans, questions, and the final answer.

#### Scenario: Active Codex turn shows layered entries
- **WHEN** the frontend receives `chat.turn.event` entries for an active Codex turn
- **THEN** the chat page renders each entry according to its `kind` with a dedicated style
- **AND** the final answer remains visually primary

#### Scenario: Completed turn keeps process details attached
- **WHEN** a Codex turn completes and the final assistant message appears in the transcript
- **THEN** the frontend keeps the structured runtime feed as a collapsible detail block below that assistant message
- **AND** historical messages without feed data continue to render normally

### Requirement: Chat UI MUST restore runtime timelines from backend snapshots
The chat page MUST restore completed and running Codex process timelines from backend turn snapshots instead of treating browser runtime state as the source of truth.

The frontend MAY still keep lightweight view state locally, but it MUST derive visible process content from backend `messages` plus persisted `turns` data.

#### Scenario: Refresh during running turn preserves pending timeline
- **WHEN** the user refreshes the chat page while a Codex-backed turn is still running
- **THEN** the page reloads the persisted running turn snapshot from the backend
- **AND** the pending assistant bubble continues showing the already persisted process timeline

#### Scenario: Refresh after completion does not create duplicate assistant bubbles
- **WHEN** the user refreshes after a turn has completed
- **THEN** the page renders one completed assistant message bubble
- **AND** the associated process details come from the persisted completed turn timeline instead of a separate stale pending bubble
