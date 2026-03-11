package repolib

import (
	"context"
	"fmt"
)

// missingEmbedder is a placeholder embedder for v1: vector retrieval is disabled
// until a real local embedder (e.g. ONNX) is wired in.
type missingEmbedder struct{}

func (m *missingEmbedder) ModelID() string { return "missing" }
func (m *missingEmbedder) Dim() int        { return 0 }
func (m *missingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	_ = text
	return nil, fmt.Errorf("embedding resources not configured")
}
