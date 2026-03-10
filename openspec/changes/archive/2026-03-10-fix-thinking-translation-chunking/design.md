## Context

Codex app-server currently drives two parallel concerns during a chat turn:

1. Structured runtime feed entries (`chat.turn.event`) for thinking, tool, plan, question, and answer.
2. Compatibility text streams (`chat.turn.delta`, `chat.turn.thinking.delta`) for legacy UI state and persisted final reasoning text.

Two issues combine here:

- Codex core/app-server compatibility can expose both structured `item/*` deltas and legacy `codex/event/*` text deltas for the same semantic content, so the daemon may append duplicated visible text.
- Thinking translation uses one global buffer and force-flushes whenever thinking is interrupted. On Codex turns, system/progress/tool events interrupt frequently, so tiny fragments like punctuation or one-word tails get translated immediately.

## Goals / Non-Goals

**Goals:**
- Remove duplicated Codex visible text caused by mixed legacy + structured content streams.
- Keep structured thinking segments split by interleaving runtime activity.
- Translate thinking in larger, coherent chunks while keeping translated output attached to the correct thinking entry.
- Preserve compatibility broadcasts and persisted translation snapshots.

**Non-Goals:**
- Redesign the chat UI presentation.
- Change non-Codex CLI streaming protocols beyond benefiting from the improved translation chunker.
- Add new settings knobs for translation chunk sizes in this change.

## Decisions

### 1. Suppress duplicate Codex legacy text notifications at the app-server connection

Use `initialize.params.capabilities.optOutNotificationMethods` to opt out of legacy Codex text notifications that duplicate structured `item/*` deltas. This is the most root-cause-oriented fix because it avoids duplicated content before the daemon needs to reconcile it.

Alternatives considered:
- **Post-parse dedupe in the daemon:** possible, but fragile because ordering between legacy and structured events is transport-dependent.
- **Frontend dedupe only:** too late; duplicated content would already pollute persistence/translation buffers.

### 2. Keep structured thinking segmentation based on contiguous reasoning runs

`codexTurnFeedEmitter` should create a new thinking entry whenever reasoning resumes after a non-thinking event, even if Codex reuses the same `itemId`. The contiguous timeline segment, not the upstream item id alone, is the correct UI/persistence boundary.

Alternatives considered:
- **Reuse the same thinking entry per `itemId`:** violates the existing structured timeline contract and makes interleaved tool/progress updates harder to read.

### 3. Switch thinking translation to entry-scoped pending buffers

Replace the single active translation buffer with per-entry pending buffers. Closed thinking entries may keep a short untranslated tail until that entry accumulates a publishable chunk or the turn completes. This preserves entry scoping without forcing low-context translation calls.

Alternatives considered:
- **Keep one global buffer and flush on every entry switch:** preserves ordering but recreates the same single-word/punctuation translation problem.
- **Merge multiple thinking entries into one translation stream:** would blur entry ownership and break runtime feed scoping.

### 4. Preserve raw whitespace inside translation buffers

Translation buffering should keep intra-sentence spaces/newlines from incoming deltas. Only outer empty checks should trim, not the stored delta payload itself.

## Risks / Trade-offs

- [Some old Codex app-server builds might rely on legacy text notifications] → Keep the legacy CLI wrapper fallback unchanged; only the app-server transport opts out of duplicate legacy text notifications.
- [Short thinking fragments may appear untranslated until later in the turn] → This is intentional; the trade-off favors coherent translation and lower helper-model frequency over immediate low-quality fragments.
- [Per-entry translation state is more complex than one buffer] → Cover with focused unit tests for entry switching, completion flush, and whitespace preservation.
