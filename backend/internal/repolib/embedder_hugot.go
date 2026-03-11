package repolib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

type hugotEmbedder struct {
	modelID string
	dim     int

	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex
}

func newHugotEmbedder(ctx context.Context, modelsDir string) (*hugotEmbedder, error) {
	_ = ctx

	modelID := strings.TrimSpace(os.Getenv("VIBE_TREE_EMBEDDER_MODEL_ID"))
	if modelID == "" {
		modelID = "KnightsAnalytics/all-MiniLM-L6-v2"
	}

	allowDownload := envBool("VIBE_TREE_EMBEDDER_ALLOW_DOWNLOAD", true)
	verbose := envBool("VIBE_TREE_EMBEDDER_VERBOSE", false)

	if override := strings.TrimSpace(os.Getenv("VIBE_TREE_EMBEDDER_MODELS_DIR")); override != "" {
		modelsDir = override
	}
	if strings.TrimSpace(modelsDir) == "" {
		return nil, fmt.Errorf("embedder: models dir not configured")
	}
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return nil, err
	}

	modelPath, err := ensureHugotModel(modelID, modelsDir, allowDownload, verbose)
	if err != nil {
		return nil, err
	}

	session, err := hugot.NewGoSession()
	if err != nil {
		return nil, err
	}

	config := hugot.FeatureExtractionConfig{
		ModelPath:    modelPath,
		Name:         "repo_library_embedder",
		OnnxFilename: "model.onnx",
		Options: []hugot.FeatureExtractionOption{
			pipelines.WithNormalization(),
		},
	}
	p, err := hugot.NewPipeline(session, config)
	if err != nil {
		_ = session.Destroy()
		return nil, err
	}

	// Infer embedding dimension from a probe call so we can support other
	// feature-extraction embedding models too.
	dim := 0
	if probe, err := p.RunPipeline([]string{"dimension probe"}); err == nil && probe != nil && len(probe.Embeddings) == 1 {
		dim = len(probe.Embeddings[0])
	}
	if dim <= 0 {
		_ = session.Destroy()
		return nil, fmt.Errorf("embedder: failed to infer embedding dim")
	}
	return &hugotEmbedder{
		modelID:  modelID,
		dim:      dim,
		session:  session,
		pipeline: p,
	}, nil
}

func (e *hugotEmbedder) Close() error {
	if e == nil || e.session == nil {
		return nil
	}
	return e.session.Destroy()
}

func (e *hugotEmbedder) ModelID() string { return e.modelID }
func (e *hugotEmbedder) Dim() int        { return e.dim }

func (e *hugotEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	if e == nil || e.pipeline == nil {
		return nil, fmt.Errorf("embedder not initialized")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	// Hugot pipelines are not documented as goroutine-safe; serialize calls.
	e.mu.Lock()
	defer e.mu.Unlock()

	out, err := e.pipeline.RunPipeline([]string{text})
	if err != nil {
		return nil, err
	}
	if out == nil || len(out.Embeddings) != 1 {
		return nil, fmt.Errorf("unexpected embedder output")
	}
	vec := out.Embeddings[0]
	if len(vec) != e.dim {
		return nil, fmt.Errorf("unexpected embedding dim: got %d want %d", len(vec), e.dim)
	}
	cp := make([]float32, len(vec))
	copy(cp, vec)
	return cp, nil
}

func ensureHugotModel(modelID string, modelsDir string, allowDownload bool, verbose bool) (string, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return "", fmt.Errorf("embedder: model id required")
	}
	modelDir := cachedHugotModelDir(modelsDir, modelID)
	if isFile(filepath.Join(modelDir, "model.onnx")) && isFile(filepath.Join(modelDir, "tokenizer.json")) {
		return modelDir, nil
	}
	if !allowDownload {
		return "", fmt.Errorf("embedder model missing at %s (set VIBE_TREE_EMBEDDER_ALLOW_DOWNLOAD=1 to auto-download)", modelDir)
	}
	opts := hugot.NewDownloadOptions()
	opts.Verbose = verbose
	path, err := hugot.DownloadModel(modelID, modelsDir, opts)
	if err != nil {
		return "", err
	}
	return path, nil
}

func cachedHugotModelDir(modelsDir string, modelID string) string {
	// Mirrors hugot.DownloadModel path derivation.
	modelP := modelID
	if strings.Contains(modelP, ":") {
		modelP = strings.Split(modelP, ":")[0]
	}
	return filepath.Join(modelsDir, strings.ReplaceAll(modelP, "/", "_"))
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
