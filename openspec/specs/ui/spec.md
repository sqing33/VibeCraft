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
The UI MUST provide at least six tabs:

- `基本设置`: contains thinking translation configuration.
- `连接与诊断`: contains daemon URL switching and diagnostics (version/paths).
- `API 来源`: contains reusable API source configuration.
- `模型设置`: contains runtime-scoped model bindings for SDK and CLI runtimes.
- `CLI 工具`: contains tool-level CLI configuration.
- `专家`: contains expert list, expert details, and AI creation workflow.

#### Scenario: User switches settings tabs
- **WHEN** user opens System Settings
- **THEN** the UI shows multiple tabs including `基本设置`, `连接与诊断`, `API 来源`, `模型设置`, `CLI 工具`, and `专家`
- **AND** switching tabs updates the visible settings content

### Requirement: Basic settings tab can configure thinking translation
The `基本设置` tab MUST provide a `思考过程翻译` settings section.

The section MUST contain exactly these configurable fields:
- `翻译模型`: a selectable SDK runtime model

The section MUST explain that the system automatically decides whether the model's thinking content needs to be translated into Chinese.

If no SDK runtime model exists, the UI MUST disable the translation configuration field and guide the user to configure a translation model first in the `模型设置` tab.

Saving the form MUST call `PUT /api/v1/settings/basic` and show success or failure feedback.

#### Scenario: Save thinking translation settings
- **WHEN** user selects a translation model and clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/basic`
- **AND** the UI shows a success toast on success

#### Scenario: Basic settings disabled before SDK model configuration
- **WHEN** the user opens `基本设置` before configuring any SDK runtime model
- **THEN** the UI disables the thinking translation field
- **AND** the UI shows guidance to configure a model in the `模型设置` tab first

### Requirement: UI can edit and save LLM settings
In the `API 来源` tab, the UI MUST organize reusable source settings as independent source cards without nested model rows.

Each Source card MUST manage:
- Source metadata: `id`, `label`, `base_url`
- Source secret: `api_key`
- Optional source-level `auth_mode` metadata for iFlow usage

The Source card MUST NOT expose a source-level provider/type selector.

The UI MUST save source changes by calling `PUT /api/v1/settings/api-sources`.

#### Scenario: User edits an API source and saves
- **WHEN** user creates or edits an API source
- **AND** user clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/api-sources`
- **AND** the request body omits any source-level provider field
- **AND** the UI shows a success toast

### Requirement: Model profiles can be tested from the settings UI
In the `模型设置` tab, each runtime model card MUST provide a `测试` action when the model binding's effective provider supports SDK test probing.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the card's effective provider resolved from the model binding and the card's model ID as the effective model name.

The UI MUST show success or failure feedback to the user.

#### Scenario: User tests a runtime model card
- **WHEN** user clicks `测试` on a runtime model card with complete API source and model configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the request uses the card's model-level provider and model ID
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

The chat page MUST support selecting runtime/model per message and, for Codex CLI turns, sending the selected `reasoning_effort` to the daemon.

#### Scenario: Send turn and receive stream
- **WHEN** user sends a message in an active chat session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/turns`
- **AND** assistant response appears incrementally as `chat.turn.delta` events arrive

#### Scenario: Send Codex turn with runtime options
- **WHEN** user selects Codex CLI, a model, and a reasoning effort
- **AND** sends a message
- **THEN** the UI includes the selected runtime/model metadata
- **AND** includes `reasoning_effort` in the turn request

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
The UI MUST provide a dedicated `CLI 工具` tab for managing `Codex CLI`, `Claude Code`, `iFlow CLI`, and `OpenCode CLI`, including enablement, optional command path override, and tool-specific health or login actions.

The `CLI 工具` tab MUST NOT be the primary editor for per-runtime model lists or default model bindings.

For `iFlow CLI`, the tab MUST expose browser-login actions and current official browser-auth status.

#### Scenario: User manages four primary CLI tools
- **WHEN** user opens System Settings
- **THEN** the UI shows a `CLI 工具` tab
- **AND** the tab allows managing `Codex CLI`, `Claude Code`, `iFlow CLI`, and `OpenCode CLI`

#### Scenario: User starts iFlow browser login from settings
- **WHEN** the user clicks the iFlow browser-login action
- **THEN** the frontend starts a daemon-managed auth session
- **AND** shows the real OAuth URL parsed from the iFlow terminal output
- **AND** allows the user to submit the returned authorization code

### Requirement: Chat UI SHALL support runtime-first model selection
The chat page MUST let the user select a conversation runtime first and then choose a compatible model from that runtime's saved model bindings.

The runtime list MUST include enabled CLI tools and available SDK runtimes in the same selector.

At minimum, when corresponding models exist, the selector MUST expose:

- `Codex CLI`
- `Claude Code`
- `iFlow CLI`
- `OpenCode CLI`
- `OpenAI SDK`
- `Anthropic SDK`

For every runtime, the model selector MUST show only the model bindings saved under that runtime.

#### Scenario: User chooses iFlow then iFlow model
- **WHEN** user selects `iFlow CLI` in the chat composer
- **THEN** the model selector only shows the models configured under runtime `iflow`
- **AND** the default value comes from that runtime's `default_model_id`

#### Scenario: User chooses OpenCode then OpenAI model
- **WHEN** user selects `OpenCode CLI` in the chat composer
- **THEN** the model selector includes only the models configured under runtime `opencode`
- **AND** selecting an OpenAI-bound model preserves that bound source at submit time

#### Scenario: Active OpenCode session restores selector state
- **WHEN** an active session stores `cli_tool_id="opencode"`
- **THEN** the chat page restores the `OpenCode CLI` runtime option and current model selection

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

### Requirement: Chat composer MUST expose Codex reasoning effort control
The chat page MUST support selecting runtime/model per message and, for Codex CLI turns, sending the selected `reasoning_effort` to the daemon.

The `reasoning_effort` selector MUST remain a Codex-only control and MUST NOT be enabled for other CLI families, including OpenCode.

#### Scenario: OpenCode runtime disables reasoning effort selector
- **WHEN** the active composer runtime is `OpenCode CLI`
- **THEN** the reasoning effort selector remains visible but disabled
- **AND** the turn request does not include `reasoning_effort`

### Requirement: Chat composer MUST use a compact right-side control rail
The chat composer MUST use a left-right layout where the text input occupies the dominant area and the control rail occupies a narrower fixed-width column.

The control rail MUST be arranged in three rows: runtime selector, model selector, and a compact row containing the reasoning effort selector plus attachment and send buttons.

#### Scenario: User opens the composer on a wide layout
- **WHEN** the chat page has enough width for the desktop composer layout
- **THEN** the left textarea occupies the remaining width and height
- **AND** the right control rail remains visually narrower than before

### Requirement: Runtime model editor MUST use simplified model cards
In the `模型设置` tab, each runtime MUST render its models as responsive cards in a multi-column grid with up to three columns.

Each model card MUST:
- show `模型` as the card title
- provide `设为默认`, `测试`, `删除` actions in the card header
- expose exactly three editable rows: `模型`, `显示名称`, `API 来源`

The `模型设置` tab MUST NOT expose:
- a runtime-level `默认模型` dropdown
- a protocol-family editor
- a separate actual-model editor

#### Scenario: User sets a runtime default from a model card
- **WHEN** the user clicks `设为默认` on a model card
- **THEN** the UI updates that runtime's `default_model_id` to the card's model id
- **AND** the runtime-level default selector is not shown elsewhere in the tab

#### Scenario: User edits a simplified model card
- **WHEN** the user edits only `模型`, `显示名称`, and `API 来源` on a model card and saves
- **THEN** the UI calls `PUT /api/v1/settings/runtime-models`
- **AND** the save succeeds without requiring separate protocol-family or actual-model inputs

### Requirement: Settings dialog MUST use unified pill navigation and compact controls
The settings dialog MUST render its tab navigation as pill-shaped controls, including both the tab list container and the active tab indicator.

Primary `Button`, `Input`, and `Select` controls inside settings tabs MUST use a compact height and full-pill rounded shape.

#### Scenario: User switches settings tabs with pill navigation
- **WHEN** the user opens settings
- **THEN** the top tab navigation renders as a pill-shaped segmented control
- **AND** the selected tab also renders with a pill-shaped active state

### Requirement: Settings tabs MUST keep main actions in a fixed footer
Each settings tab MUST keep its primary actions in a footer region that remains visible while the tab content scrolls vertically.

The tab body MUST be the only vertical scroll area for long settings content.

#### Scenario: User scrolls a long settings tab
- **WHEN** the tab content exceeds the available height
- **THEN** only the content area scrolls vertically
- **AND** the main action footer remains fixed at the bottom of the tab panel

### Requirement: Chat page MUST support importing Codex CLI history
The chat page MUST continue to render existing local chat sessions and MUST additionally expose a frontend entry for importing Codex CLI history.

The import entry MUST be available from the chat session list area and MUST open a selection dialog that shows readable history titles returned by the backend.

The dialog MUST allow users to:
- search or filter the thread list by title
- select one or more history entries
- start the import

After a successful import, the UI MUST refresh the chat session list and SHOULD switch to the first newly imported session.

#### Scenario: Import Codex history from the chat page
- **WHEN** the user clicks the chat page import entry and confirms selected Codex history threads
- **THEN** the UI calls the backend import API
- **AND** the session list refreshes with the newly imported sessions
- **AND** imported sessions are shown with readable titles instead of raw thread ids

### Requirement: Chat UI MUST catch up running turns when live updates stall
When a chat turn is running, the UI MUST avoid requiring a manual page refresh to see newly persisted turn timeline progress.

If the WebSocket connection is disconnected, or if chat live-update events stop arriving for a sustained period while a turn is still pending, the UI MUST poll the backend for persisted turn snapshots and update the visible pending timeline accordingly.

#### Scenario: Stale WebSocket triggers turns polling catch-up
- **WHEN** a chat session has a pending turn (assistant is still running)
- **AND** the WebSocket is disconnected OR no `chat.turn.*` live-update events are received for a sustained period
- **THEN** the UI polls `GET /api/v1/chat/sessions/:id/turns`
- **AND** the pending process timeline advances to reflect newly persisted timeline items without a full page refresh

#### Scenario: Catch-up polling stops when the turn becomes terminal
- **WHEN** the UI is polling for catch-up during a pending turn
- **AND** the backend snapshot shows the latest turn is no longer running
- **THEN** the UI stops catch-up polling
- **AND** the chat page converges to a completed assistant message bubble with attached process details

