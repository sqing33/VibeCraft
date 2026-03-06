## 1. Spec and data model

- [x] 1.1 Extend expert/config data structures for metadata, model refs, and fallback settings
- [x] 1.2 Add config helpers to rebuild llm-model experts and hydrate custom expert runtime fields
- [x] 1.3 Add/update tests covering config merge and fallback-related validation

## 2. Backend expert settings and generation APIs

- [x] 2.1 Add expert settings read/save endpoints with safe response types
- [x] 2.2 Implement expert builder service, schema, and demo generation path
- [x] 2.3 Add API tests for expert settings load/save/generate flows

## 3. Runtime fallback execution

- [x] 3.1 Extend runner/expert resolve output to carry SDK fallback alternatives
- [x] 3.2 Implement SDK execution fallback in runner and chat manager
- [x] 3.3 Add tests for primary failure to secondary success behavior

## 4. UI expert settings experience

- [x] 4.1 Add ExpertSettingsTab and wire it into SettingsDialog after the 模型 tab
- [x] 4.2 Implement expert list/detail display with status and metadata badges
- [x] 4.3 Implement AI 创建专家 modal with conversation, draft preview, publish, toggle, and delete actions

## 5. Docs and archive

- [x] 5.1 Add project skill doc for expert-creator and update PROJECT_STRUCTURE.md
- [x] 5.2 Run targeted tests for backend and frontend build verification
- [x] 5.3 Mark tasks complete and archive the change after implementation
