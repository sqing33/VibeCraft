package searchdb

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

type fakeEmbedder struct{}

func (f *fakeEmbedder) ModelID() string { return "fake" }
func (f *fakeEmbedder) Dim() int        { return 3 }
func (f *fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "alpha"):
		return []float32{1, 0, 0}, nil
	case strings.Contains(text, "bravo"):
		return []float32{0, 1, 0}, nil
	case strings.Contains(text, "charlie"):
		return []float32{0, 0, 1}, nil
	default:
		return []float32{0, 0, 0}, nil
	}
}

func TestVectorSearchBasic(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "search.db")

	sdb, err := Open(context.Background(), OpenParams{DBPath: dbPath, Embedder: &fakeEmbedder{}})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sdb.Close()

	if !sdb.VecEnabled() {
		// Best-effort: enable vec (downloads extension when allowed). If the
		// environment disallows network/dynamic loading, skip instead of failing.
		if err := sdb.tryLoadVecExtension(context.Background()); err != nil {
			t.Skipf("vector extension unavailable: %v", err)
		}
		if !sdb.VecEnabled() {
			t.Skip("vector search still disabled")
		}
	}

	chunks := []Chunk{
		{
			ChunkID:      "card:alpha",
			RepoSourceID: "rs_demo",
			AnalysisID:   "ra_demo",
			SourceKind:   "card",
			SourceRefID:  "card:alpha",
			Title:        "Alpha",
			DisplayText:  "Alpha",
			SearchText:   "alpha",
			TextExcerpt:  "alpha",
		},
		{
			ChunkID:      "card:bravo",
			RepoSourceID: "rs_demo",
			AnalysisID:   "ra_demo",
			SourceKind:   "card",
			SourceRefID:  "card:bravo",
			Title:        "Bravo",
			DisplayText:  "Bravo",
			SearchText:   "bravo",
			TextExcerpt:  "bravo",
		},
	}

	if err := sdb.UpsertChunks(context.Background(), chunks); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	hits, err := sdb.SearchVector(context.Background(), "alpha", 5, []string{"ra_demo"}, []string{"card"})
	if err != nil {
		t.Fatalf("search vector: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected hits")
	}
	if hits[0].ChunkID != "card:alpha" {
		t.Fatalf("expected top hit alpha, got %s", hits[0].ChunkID)
	}
}
