package searchdb

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSearchTitleMatchesPrefersExactTitleSubstring(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "search.db")

	sdb, err := Open(context.Background(), OpenParams{DBPath: dbPath, Embedder: &fakeEmbedder{}})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sdb.Close()

	chunks := []Chunk{
		{
			ChunkID:      "card:characteristic",
			RepoSourceID: "rs_demo",
			AnalysisID:   "ra_demo",
			SourceKind:   "card",
			Title:        "会话隔离的多 Agent 委派与实例池",
			DisplayText:  "x",
			SearchText:   "多 Agent 并行协作",
			TextExcerpt:  "x",
		},
		{
			ChunkID:      "card:question",
			RepoSourceID: "rs_demo",
			AnalysisID:   "ra_demo",
			SourceKind:   "card",
			Title:        "多 Agent 并行机制：任务拆分、调度、路由/接力、结果合并/冲突处理",
			DisplayText:  "x",
			SearchText:   "多 Agent 并行机制",
			TextExcerpt:  "x",
		},
	}
	if err := sdb.UpsertChunks(context.Background(), chunks); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	hits, err := sdb.SearchTitleMatches(context.Background(), "多 Agent 并行机制", 8, []string{"ra_demo"}, []string{"card"})
	if err != nil {
		t.Fatalf("search title matches: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected hits")
	}
	if hits[0].ChunkID != "card:question" {
		t.Fatalf("expected exact title hit first, got %s", hits[0].ChunkID)
	}
}
