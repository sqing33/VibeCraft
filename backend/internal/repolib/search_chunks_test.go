package repolib

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/repolib/searchdb"
	"vibe-tree/backend/internal/store"
)

func TestBuildSearchChunksKeepsReportChunkIDsStable(t *testing.T) {
	report := `# GitHub 功能实现原理报告

## Run 1

## 第一部分：技术栈与模块语言
- 仓库: x
- 请求 Ref: main
- 解析 Ref: main
- Commit: abc
- 生成时间: 2026-03-11
- 主要语言/技术栈总览: Go
- 后端: Go
- 前端: React
- 其它模块: -

## 第二部分：项目用途与核心特点
- 项目做什么用: x
- 典型使用场景: x
- 核心特点概览: x

### 风险与局限
- x

## 第三部分：特点实现思路
### 特点 1: 多 Agent 并行机制
- 动机: x
- 目标: x
- 思路: x
- 取舍: x
- 置信度: high

## 第四部分：提问与解答
### 问题 1: 多 Agent 并行机制
- 结论: 任务拆分、调度和结果合并由同一套编排链路完成。
- 思路: 先拆任务，再调度，再合并。
- 取舍: 牺牲一点延迟换取稳定性。
- 置信度: high

## 第五部分：实现定位与证据
### 问题 1: 多 Agent 并行机制
- src/agent/router.go:10 [control-flow] - route
	`

		source := store.RepoSource{ID: "rs_demo", RepoURL: "https://github.com/a/b", RepoKey: "a-b"}
		analysis := store.RepoAnalysisResult{ID: "ra_demo"}
		line := int64(10)
		card := store.RepoKnowledgeCard{
			ID:             "rc_demo",
			RepoSourceID:   source.ID,
			AnalysisID:     analysis.ID,
			Title:          "多 Agent 并行机制",
			CardType:       "feature_pattern",
			Summary:        "先拆任务，再调度，再合并。",
			SectionTitle:   pointer("问题 1: 多 Agent 并行机制"),
		}
	evidenceByCard := map[string][]store.RepoKnowledgeEvidence{
		card.ID: {
			{
				ID:        "re_demo",
				CardID:    card.ID,
				Path:      "src/agent/router.go",
				Line:      &line,
				SortIndex: 1,
			},
		},
		}

		chunks1 := buildSearchChunks(source, analysis, report, []store.RepoKnowledgeCard{card}, evidenceByCard)
		chunks2 := buildSearchChunks(source, analysis, report, []store.RepoKnowledgeCard{card}, evidenceByCard)
		if len(chunks1) == 0 || len(chunks2) == 0 {
			t.Fatalf("expected chunks")
		}
	ids1 := map[string]struct{}{}
	for _, chunk := range chunks1 {
		ids1[chunk.ChunkID] = struct{}{}
	}
	for _, chunk := range chunks2 {
		if _, ok := ids1[chunk.ChunkID]; !ok {
			t.Fatalf("chunk_id not stable: %s", chunk.ChunkID)
		}
	}
}

func TestBuildSearchChunksMapsReportAndEvidenceBackToCard(t *testing.T) {
	report := `# GitHub 功能实现原理报告

## Run 1

## 第一部分：技术栈与模块语言
- 仓库: x

## 第二部分：项目用途与核心特点
- 项目做什么用: x

## 第三部分：特点实现思路
### 特点 1: 多 Agent 并行机制
- 动机: x
- 目标: x

## 第四部分：提问与解答
### 问题 1: 多 Agent 并行机制
- 结论: 编排层统一负责拆分与合并。
- 思路: 先拆分，再调度。

## 第五部分：实现定位与证据
### 问题 1: 多 Agent 并行机制
- src/agent/router.go:10 [control-flow] - route
	`

		source := store.RepoSource{ID: "rs_demo", RepoURL: "https://github.com/a/b", RepoKey: "a-b"}
		analysis := store.RepoAnalysisResult{ID: "ra_demo"}
		line := int64(10)
		conclusion := "编排层统一负责拆分与合并。"
		card := store.RepoKnowledgeCard{
			ID:             "rc_demo",
			RepoSourceID:   source.ID,
			AnalysisID:     analysis.ID,
			Title:          "多 Agent 并行机制",
			CardType:       "feature_pattern",
			Conclusion:     &conclusion,
			Summary:        "先拆分，再调度。",
			SectionTitle:   pointer("问题 1: 多 Agent 并行机制"),
		}
	evidenceByCard := map[string][]store.RepoKnowledgeEvidence{
		card.ID: {
			{
				ID:        "re_demo",
				CardID:    card.ID,
				Path:      "src/agent/router.go",
				Line:      &line,
				SortIndex: 1,
			},
			},
		}

		chunks := buildSearchChunks(source, analysis, report, []store.RepoKnowledgeCard{card}, evidenceByCard)
		if len(chunks) != 3 {
			t.Fatalf("expected report/card/evidence chunks, got %d", len(chunks))
		}
	foundReport := false
	foundEvidence := false
	for _, chunk := range chunks {
		switch chunk.SourceKind {
		case "report_section":
			foundReport = true
			if chunk.SourceRefID != card.ID {
				t.Fatalf("expected report chunk to point to card %s, got %s", card.ID, chunk.SourceRefID)
			}
		case "evidence":
			foundEvidence = true
			if chunk.SourceRefID != card.ID {
				t.Fatalf("expected evidence chunk to point to card %s, got %s", card.ID, chunk.SourceRefID)
			}
			if chunk.SearchText == "" || !containsAll(chunk.SearchText, "CardTitle: 多 Agent 并行机制", "CardConclusion: "+conclusion) {
				t.Fatalf("expected evidence chunk search text to include card context, got %q", chunk.SearchText)
			}
		}
	}
	if !foundReport {
		t.Fatalf("expected report_section chunk")
	}
	if !foundEvidence {
		t.Fatalf("expected evidence chunk")
	}
}

func TestSearchDBKeywordWorksWithoutVec(t *testing.T) {
	t.Setenv("VIBE_TREE_SQLITE_TEST_TAGS", "sqlite_fts5")
	root := t.TempDir()
	_ = os.Setenv("XDG_DATA_HOME", root)

	repoDir, err := paths.RepoLibraryDir()
	if err != nil {
		t.Fatalf("RepoLibraryDir: %v", err)
	}
	dbPath := filepath.Join(repoDir, "search", "search.db")
	if err := paths.EnsureDir(filepath.Dir(dbPath)); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	sdb, err := searchdb.Open(context.Background(), searchdb.OpenParams{DBPath: dbPath, Embedder: &missingEmbedder{}})
	if err != nil {
		t.Fatalf("open searchdb: %v", err)
	}
	defer sdb.Close()

	if sdb.VecEnabled() {
		t.Fatalf("expected vec disabled")
	}
	chunks := []searchdb.Chunk{
		{
			ChunkID:      "card:rc_demo",
			RepoSourceID: "rs_demo",
			AnalysisID:   "ra_demo",
			SourceKind:   "card",
			SourceRefID:  "rc_demo",
			Title:        "Circuit Breaker",
			DisplayText:  "Circuit Breaker\n\nhalf-open",
			SearchText:   "Circuit Breaker\nhalf-open circuit breaker",
			TextExcerpt:  "Circuit Breaker half-open",
		},
	}
	if err := sdb.UpsertChunks(context.Background(), chunks); err != nil {
		t.Fatalf("upsert chunks: %v", err)
	}
	hits, err := sdb.SearchKeyword(context.Background(), "half-open", 10, nil, nil)
	if err != nil {
		t.Fatalf("keyword search: %v", err)
	}
	if len(hits) == 0 {
		hits, err = sdb.SearchKeyword(context.Background(), "\"half-open\"", 10, nil, nil)
		if err != nil {
			t.Fatalf("keyword search (quoted): %v", err)
		}
	}
	if len(hits) == 0 {
		t.Fatalf("expected hits")
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
