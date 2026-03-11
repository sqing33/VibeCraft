## 1. Report Context Summary

- [x] 1.1 Add a Go parser that extracts the formal report's technology-stack summary fields from the first report section
- [x] 1.2 Expose the derived summary in Repo Library repository detail data for the selected snapshot

## 2. Repository Detail UI Layout

- [x] 2.1 Replace snapshot and analysis card lists with compact selectors in the repository detail page
- [x] 2.2 Rebuild repository detail into a fixed three-column workspace with pane-scoped scrolling
- [x] 2.3 Move full report reading behind a `查看报告` action with a dedicated reading surface

## 3. Detail Context Presentation

- [x] 3.1 Render the derived technology-stack summary in the left column as repository context
- [x] 3.2 Keep card list browsing in the center pane and card summary fixed above a scrollable evidence region in the right pane

## 4. Verification & Docs

- [x] 4.1 Add or update tests for the report context summary extraction behavior
- [x] 4.2 Validate the updated repository detail page with a production UI build
- [x] 4.3 Update `PROJECT_STRUCTURE.md` to reflect the new detail workspace behavior and summary extraction responsibility
