## 1. Schema and storage

- [x] 1.1 Add chat attachment SQLite migration and store types
- [x] 1.2 Add chat attachment file storage helpers and validation rules

## 2. Backend chat flow

- [x] 2.1 Extend chat turn API to accept multipart attachment requests
- [x] 2.2 Persist attachments and feed provider-native multimodal inputs in chat manager
- [x] 2.3 Reconstruct attachment-bearing history safely and skip automatic compaction for such sessions

## 3. Frontend chat flow

- [x] 3.1 Extend chat API client and store to submit FormData turns with attachments
- [x] 3.2 Add chat composer attachment selection/removal UI and render attachments in message history

## 4. Validation and docs

- [x] 4.1 Add or update backend tests for attachment turns, storage, and reconstruction
- [x] 4.2 Update `PROJECT_STRUCTURE.md`, run focused validation, and archive the change
