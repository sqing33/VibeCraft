## 1. Codex stream dedupe

- [x] 1.1 Opt out of duplicate legacy Codex text notifications during app-server initialization.
- [x] 1.2 Keep Codex structured thinking segments split across interleaved non-thinking activity.

## 2. Thinking translation chunking

- [x] 2.1 Refactor thinking translation runtime to use entry-scoped pending buffers.
- [x] 2.2 Preserve whitespace and defer undersized closed fragments until publishable or complete.

## 3. Regression coverage

- [x] 3.1 Add tests for Codex same-item interleaving and stream dedupe behavior.
- [x] 3.2 Add tests for coherent thinking translation chunking and final completion flush.
