# repo-library-search Specification

## Purpose

Repo Library search 定义基于向量索引与结构化过滤的语义检索能力，使用户可以跨仓库查找相似实现模式。

## ADDED Requirements

### Requirement: Repo Library MUST build a searchable index from stored analyses
The system MUST build and refresh a searchable index from stored Repo Library analyses.

The index MUST include text derived from reports and extracted knowledge cards, and MUST support incremental refresh after new analyses complete.

#### Scenario: Index refresh after a successful analysis
- **WHEN** a repository analysis finishes successfully
- **THEN** the system updates the search index using the new analysis outputs
- **AND** the new repository becomes searchable without rebuilding unrelated entries

### Requirement: Repo Library MUST support semantic search queries
The system MUST provide a search API that accepts a natural-language query and returns ranked results with relevance information.

The search API MUST support optional repository filters and result limits.

#### Scenario: User searches for a feature pattern
- **WHEN** a client submits `POST /api/v1/repo-library/search` with a query such as “认证流程”
- **THEN** the system returns ranked matches across repositories
- **AND** each result includes enough summary information for the user to decide whether to inspect it further

#### Scenario: User restricts search to selected repositories
- **WHEN** a client includes repository filters in the search request
- **THEN** the backend limits the search corpus to those repositories
- **AND** excludes results outside the selected filter set

### Requirement: Repo Library search results MUST be traceable
Each search result MUST preserve a reference back to its repository, snapshot, and related card or chunk so users can inspect the source context.

#### Scenario: User opens a result from pattern search
- **WHEN** the UI selects a returned search result
- **THEN** the backend-provided payload includes repository and snapshot identifiers
- **AND** the UI can navigate to repository detail or card detail without guessing source context
