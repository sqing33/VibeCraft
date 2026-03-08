## 1. OpenSpec and data model

- [x] 1.1 Rewrite MCP/Skill OpenSpec requirements for JSON-native MCP and discovered Skill catalog
- [x] 1.2 Refactor MCP config model to drop redundant enable bindings and preserve raw JSON
- [x] 1.3 Refactor Skill runtime model to rely on discovery instead of persisted bindings

## 2. Backend and runtime

- [x] 2.1 Update MCP settings API to parse wrapped/flat JSON and return editable raw JSON
- [x] 2.2 Simplify Skill settings API to discovered catalog output only
- [x] 2.3 Update Codex runtime effective MCP and Skill injection logic to match the new model

## 3. Frontend and validation

- [x] 3.1 Rebuild MCP settings tab around JSON import/edit and default-enabled toggles only
- [x] 3.2 Rebuild Skill settings tab as discovered catalog/status view
- [x] 3.3 Update chat session MCP selectors and run focused backend/frontend validation
