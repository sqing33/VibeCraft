## 1. OpenSpec Alignment

- [x] 1.1 Confirm the chat page layout change against existing `ui` capability and new immersive layout capability
- [x] 1.2 Keep proposal/design/specs in sync with the implemented chat page structure

## 2. Chat Page Layout Refactor

- [x] 2.1 Adjust the `#/chat` page shell to support a narrow left session rail and a dominant right conversation workspace
- [x] 2.2 Reorganize the session rail so creation, session list, and primary actions fit the narrower column without losing core actions
- [x] 2.3 Rebuild the conversation workspace into lightweight header, scrollable transcript area, and bottom-anchored composer area
- [x] 2.4 Constrain transcript content width to a centered readable column while preserving wider rendering for long-form content when needed

## 3. Visual Polish & Verification

- [x] 3.1 Tune spacing, border, and background density to match the lower-noise immersive style
- [x] 3.2 Verify attachment chips, thinking blocks, streaming bubbles, and session actions still render correctly in the new layout
- [x] 3.3 Run frontend build verification and manually inspect `#/chat` in the Vite dev server
