package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"vibecraft/backend/internal/store"
)

func TestExpertBuilderStore_CRUD(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateExpertBuilderSession(context.Background(), store.CreateExpertBuilderSessionParams{
		Title:          "UI 专家优化",
		BuilderModelID: "design-main",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.ID == "" {
		t.Fatalf("expected session id")
	}

	if _, err := st.AppendExpertBuilderMessage(context.Background(), sess.ID, "user", "先创建一个 UI 专家"); err != nil {
		t.Fatalf("append user message: %v", err)
	}
	if _, err := st.AppendExpertBuilderMessage(context.Background(), sess.ID, "assistant", "我先给你一个草稿"); err != nil {
		t.Fatalf("append assistant message: %v", err)
	}

	snapshot, err := st.CreateExpertBuilderSnapshot(context.Background(), store.CreateExpertBuilderSnapshotParams{
		SessionID:        sess.ID,
		AssistantMessage: "我先给你一个草稿",
		DraftJSON:        `{"id":"ui-expert","label":"UI 专家","description":"负责界面设计","category":"design","primary_model_id":"design-main","system_prompt":"你是一名 UI 专家。"}`,
		Warnings:         []string{"demo warning"},
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("unexpected snapshot version: %d", snapshot.Version)
	}

	messages, err := st.ListExpertBuilderMessages(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	snapshots, err := st.ListExpertBuilderSnapshots(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	patched, err := st.PatchExpertBuilderSession(context.Background(), sess.ID, store.PatchExpertBuilderSessionParams{TargetExpertID: ptr("ui-expert"), Status: ptr("published"), LatestSnapshotID: &snapshot.ID})
	if err != nil {
		t.Fatalf("patch session: %v", err)
	}
	if patched.TargetExpertID == nil || *patched.TargetExpertID != "ui-expert" {
		t.Fatalf("unexpected target expert id: %+v", patched.TargetExpertID)
	}
	if patched.Status != "published" {
		t.Fatalf("unexpected status: %s", patched.Status)
	}
	if patched.LatestSnapshotID == nil || *patched.LatestSnapshotID != snapshot.ID {
		t.Fatalf("unexpected latest snapshot: %+v", patched.LatestSnapshotID)
	}
}

func ptr(v string) *string { return &v }
