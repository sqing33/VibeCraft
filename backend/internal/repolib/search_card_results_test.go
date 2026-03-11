package repolib

import (
	"context"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/repolib/searchdb"
	"vibe-tree/backend/internal/store"
)

func TestCollapseSearchHitsToCardsMergesSourcesAndNormalizesDisplayScore(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	source, err := st.UpsertRepoSource(ctx, store.UpsertRepoSourceParams{
		RepoURL: "https://github.com/openakita/openakita",
		Owner:   "openakita",
		Repo:    "openakita",
		RepoKey: "openakita-openakita",
	})
	if err != nil {
		t.Fatalf("upsert repo source: %v", err)
	}
	snapshot, err := st.CreateRepoSnapshot(ctx, store.CreateRepoSnapshotParams{
		SnapshotID:   "rp_demo",
		RepoSourceID: source.ID,
		RequestedRef: "main",
		StoragePath:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	run, err := st.CreateRepoAnalysisRun(ctx, store.CreateRepoAnalysisRunParams{
		RepoSourceID:   source.ID,
		RepoSnapshotID: snapshot.ID,
		Language:       "zh",
		Depth:          "standard",
		AgentMode:      "single",
		Features:       []string{"多 Agent 并行机制", "日志链路"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	line := int64(42)
	if err := st.ReplaceRepoKnowledge(ctx, store.ReplaceRepoKnowledgeParams{
		RepoSourceID:   source.ID,
		RepoSnapshotID: snapshot.ID,
		AnalysisRunID:  run.ID,
		Cards: []store.RepoKnowledgeCardInput{
			{
				Title:        "多 Agent 并行机制",
				CardType:     "feature_pattern",
				Conclusion:   pointer("统一编排层负责拆分与合并。"),
				Summary:      "主 Agent 拆分任务并回收结果。",
				SectionTitle: pointer("问题 1: 多 Agent 并行机制"),
				Evidence: []store.RepoKnowledgeEvidenceInput{
					{Path: "internal/orchestrator/router.go", Line: &line},
				},
			},
			{
				Title:      "日志链路",
				CardType:   "feature_pattern",
				Conclusion: pointer("日志按 execution 聚合。"),
				Summary:    "按执行实例回放。",
			},
		},
	}); err != nil {
		t.Fatalf("replace repo knowledge: %v", err)
	}
	cards, err := st.ListRepoCards(ctx, store.ListRepoCardsParams{RepoSourceID: source.ID, RepoSnapshotID: snapshot.ID, AnalysisRunID: run.ID, Limit: 10})
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	cardA := cards[0]
	cardB := cards[1]
	if cardA.Title != "多 Agent 并行机制" {
		cardA, cardB = cardB, cardA
	}

	svc := &Service{store: st}
	results := svc.collapseSearchHitsToCards(ctx, []searchdb.Hit{
		{
			ChunkID:        "report:" + snapshot.ID + ":a",
			SourceKind:     "report_section",
			SourceRefID:    cardA.ID,
			RepoSourceID:   source.ID,
			RepoSnapshotID: snapshot.ID,
			AnalysisRunID:  run.ID,
			Score:          1.32,
		},
		{
			ChunkID:        "card:" + cardA.ID,
			SourceKind:     "card",
			SourceRefID:    cardA.ID,
			RepoSourceID:   source.ID,
			RepoSnapshotID: snapshot.ID,
			AnalysisRunID:  run.ID,
			Score:          1.18,
		},
		{
			ChunkID:        "evidence:re_demo",
			SourceKind:     "evidence",
			SourceRefID:    cardA.ID,
			RepoSourceID:   source.ID,
			RepoSnapshotID: snapshot.ID,
			AnalysisRunID:  run.ID,
			Score:          0.74,
		},
		{
			ChunkID:        "card:" + cardB.ID,
			SourceKind:     "card",
			SourceRefID:    cardB.ID,
			RepoSourceID:   source.ID,
			RepoSnapshotID: snapshot.ID,
			AnalysisRunID:  run.ID,
			Score:          0.88,
		},
	}, 5)
	if len(results) != 2 {
		t.Fatalf("expected 2 card results, got %d", len(results))
	}
	if results[0].Card.ID != cardA.ID {
		t.Fatalf("expected multi-source card first, got %s", results[0].Card.ID)
	}
	if len(results[0].Evidence) != 1 {
		t.Fatalf("expected evidence preview to be loaded")
	}
	if got, want := results[0].MatchSources, []string{"card", "report_section", "evidence"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("unexpected match source order: %v", got)
	}
	if results[0].RawScore <= results[1].RawScore {
		t.Fatalf("expected first raw score > second: %v <= %v", results[0].RawScore, results[1].RawScore)
	}
	for _, item := range results {
		if item.DisplayScore < 1 || item.DisplayScore > 98 {
			t.Fatalf("display score out of range: %v", item.DisplayScore)
		}
	}
}
