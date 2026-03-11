package repolib

import (
	"testing"

	"vibe-tree/backend/internal/repolib/searchdb"
)

func TestSearchQueryTokensIncludesHanBigramsAndASCII(t *testing.T) {
	tokens := searchQueryTokens("多 Agent 并行机制")
	want := []string{"agent", "并行机制", "并行", "行机", "机制"}
	for _, item := range want {
		found := false
		for _, token := range tokens {
			if token == item {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected token %q in %v", item, tokens)
		}
	}
}

func TestRerankSearchHitsPrefersCardWithStrongerTitleAndExcerptOverlap(t *testing.T) {
	hits := []searchdb.Hit{
		{
			ChunkID:     "card:relevant",
			SourceKind:  "card",
			Title:       "会话隔离的多 Agent 委派与实例池",
			TextExcerpt: "支持主 Agent 委派、并行协作、临时分身和失败降级。",
			Score:       0.34,
		},
		{
			ChunkID:     "card:broad",
			SourceKind:  "card",
			Title:       "总体运行时架构：agent orchestration 总览",
			TextExcerpt: "Python 主运行时承担多 Agent 编排中心的角色。",
			Score:       0.35,
		},
		{
			ChunkID:     "report:section",
			SourceKind:  "report_section",
			Title:       "第三部分：特点实现思路 > 特点 2: 会话隔离的多 Agent 委派与实例池",
			TextExcerpt: "支持并行协作和临时分身。",
			Score:       0.33,
		},
	}

	ranked := rerankSearchHits("多 Agent 并行机制", hits)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 hits, got %d", len(ranked))
	}
	if ranked[0].ChunkID != "card:relevant" {
		t.Fatalf("expected relevant card first, got %s", ranked[0].ChunkID)
	}
	if ranked[1].ChunkID == "card:relevant" {
		t.Fatalf("expected reordered hits after first position")
	}
}
