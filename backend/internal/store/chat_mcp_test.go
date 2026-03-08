package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/store"
)

func TestChatSession_MCPServerIDsRoundTrip(t *testing.T) {
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "mcp",
		ExpertID:      "codex",
		Provider:      "cli",
		Model:         "gpt-5-codex",
		WorkspacePath: ".",
		MCPServerIDs:  []string{"filesystem", "github"},
	})
	if err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	if len(sess.MCPServerIDs) != 2 {
		t.Fatalf("unexpected create ids: %#v", sess.MCPServerIDs)
	}

	updated, err := st.PatchChatSession(context.Background(), sess.ID, store.PatchChatSessionParams{MCPServerIDs: &[]string{"github"}})
	if err != nil {
		t.Fatalf("patch chat session: %v", err)
	}
	if len(updated.MCPServerIDs) != 1 || updated.MCPServerIDs[0] != "github" {
		t.Fatalf("unexpected patched ids: %#v", updated.MCPServerIDs)
	}

	loaded, err := st.GetChatSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get chat session: %v", err)
	}
	if len(loaded.MCPServerIDs) != 1 || loaded.MCPServerIDs[0] != "github" {
		t.Fatalf("unexpected loaded ids: %#v", loaded.MCPServerIDs)
	}

	forked, err := st.ForkChatSession(context.Background(), sess.ID, "")
	if err != nil {
		t.Fatalf("fork chat session: %v", err)
	}
	if len(forked.MCPServerIDs) != 1 || forked.MCPServerIDs[0] != "github" {
		t.Fatalf("unexpected forked ids: %#v", forked.MCPServerIDs)
	}
}
