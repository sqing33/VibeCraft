package searchdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	schemaVersion        = 1
	chunkStrategyVersion = "v1"
	scoringVersion       = "v1"
)

type Meta struct {
	SchemaVersion        int    `json:"schema_version"`
	EmbedderModelID      string `json:"embedder_model_id"`
	EmbedderDim          int    `json:"embedder_dim"`
	ChunkStrategyVersion string `json:"chunk_strategy_version"`
	ScoringVersion       string `json:"scoring_version"`
	BuiltAt              int64  `json:"built_at"`
}

type Chunk struct {
	ChunkID        string
	RepoSourceID   string
	RepoSnapshotID string
	AnalysisRunID  string
	SourceKind     string
	SourceRefID    string
	Title          string
	DisplayText    string
	SearchText     string
	TagsFlat       string
	SymbolsFlat    string
	EvidenceRefs   string
	TextExcerpt    string
	ContentHash    string
	UpdatedAt      int64
}

type Hit struct {
	ChunkID        string   `json:"chunk_id"`
	Score          float64  `json:"score"`
	SourceKind     string   `json:"source_kind"`
	Title          string   `json:"title"`
	TextExcerpt    string   `json:"text_excerpt"`
	RepoSourceID   string   `json:"repository_id"`
	RepoSnapshotID string   `json:"snapshot_id"`
	AnalysisRunID  string   `json:"analysis_run_id"`
	SourceRefID    string   `json:"source_ref_id,omitempty"`
	EvidenceRefs   []string `json:"evidence_refs,omitempty"`
}

type Embedder interface {
	ModelID() string
	Dim() int
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Service struct {
	db           *sql.DB
	dbPath       string
	embedder     Embedder
	vecExtension string
	ftsEnabled   bool
	vecEnabled   bool
}

type OpenParams struct {
	DBPath       string
	Embedder     Embedder
	VecExtension string // optional; when present, load extension and enable vector search
}

func Open(ctx context.Context, params OpenParams) (*Service, error) {
	if strings.TrimSpace(params.DBPath) == "" {
		return nil, fmt.Errorf("searchdb: DBPath is required")
	}
	if err := os.MkdirAll(filepath.Dir(params.DBPath), 0o755); err != nil {
		return nil, err
	}
	ensureDriverRegistered()
	// go-sqlite3 DSN. We keep pragmas via Exec for portability.
	db, err := sql.Open(driverName, params.DBPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := pingWithTimeout(ctx, db, 3*time.Second); err != nil {
		_ = db.Close()
		return nil, err
	}

	svc := &Service{
		db:           db,
		dbPath:       params.DBPath,
		embedder:     params.Embedder,
		vecExtension: strings.TrimSpace(params.VecExtension),
		ftsEnabled:   false,
		vecEnabled:   false,
	}
	if err := svc.applyPragmas(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := svc.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := svc.tryLoadVecExtension(ctx); err != nil {
		// Non-fatal: keyword-only is still usable.
		svc.vecEnabled = false
	}
	return svc, nil
}

func (s *Service) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Service) VecEnabled() bool {
	if s == nil {
		return false
	}
	return s.vecEnabled && s.embedder != nil && s.embedder.Dim() > 0
}

func (s *Service) UpsertChunks(ctx context.Context, chunks []Chunk) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("searchdb: not initialized")
	}
	if len(chunks) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.ChunkID) == "" || strings.TrimSpace(chunk.RepoSnapshotID) == "" {
			continue
		}
		if chunk.UpdatedAt == 0 {
			chunk.UpdatedAt = time.Now().UnixMilli()
		}
		if strings.TrimSpace(chunk.ContentHash) == "" {
			chunk.ContentHash = sha256Hex(chunk.SearchText)
		}
		rowid, err := upsertChunkRow(ctx, tx, chunk)
		if err != nil {
			return err
		}
		if err := upsertFTSRow(ctx, tx, rowid, chunk); err != nil {
			return err
		}
		if s.VecEnabled() && (chunk.SourceKind == "card" || chunk.SourceKind == "report_section") {
			embedding, err := s.embedder.Embed(ctx, chunk.SearchText)
			if err != nil {
				continue
			}
			if err := upsertVecRow(ctx, tx, rowid, chunk.RepoSnapshotID, chunk.SourceKind, embedding); err != nil {
				continue
			}
		}
	}

	return tx.Commit()
}

func (s *Service) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("searchdb: not initialized")
	}
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return fmt.Errorf("searchdb: snapshot_id required")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM kb_chunks WHERE repo_snapshot_id = ?;`, snapshotID)
	return err
}

func (s *Service) SearchKeyword(ctx context.Context, query string, topK int, repoSnapshotFilters []string, sourceKinds []string) ([]Hit, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("searchdb: not initialized")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("searchdb: query required")
	}
	if !s.ftsEnabled {
		return s.searchKeywordFallback(ctx, query, topK, repoSnapshotFilters, sourceKinds)
	}
	if topK <= 0 {
		topK = 10
	}
	limit := stableRecallLimit(topK)
	where, args := buildFilterWhere(repoSnapshotFilters, sourceKinds)

	sqlText := fmt.Sprintf(`
SELECT
  c.chunk_id,
  c.source_kind,
  c.title,
  c.text_excerpt,
  c.repo_source_id,
  c.repo_snapshot_id,
  c.analysis_run_id,
  c.source_ref_id,
  c.evidence_refs_flat,
  (1.0 / (1.0 + bm25(kb_chunks_fts))) AS score
FROM kb_chunks_fts
JOIN kb_chunks c ON c.rowid = kb_chunks_fts.rowid
WHERE kb_chunks_fts MATCH ? %s
ORDER BY bm25(kb_chunks_fts) ASC
LIMIT ?;`, where)

	args = append([]any{query}, args...)
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []Hit{}
	for rows.Next() {
		var h Hit
		var refs sql.NullString
		var sourceRef sql.NullString
		if err := rows.Scan(&h.ChunkID, &h.SourceKind, &h.Title, &h.TextExcerpt, &h.RepoSourceID, &h.RepoSnapshotID, &h.AnalysisRunID, &sourceRef, &refs, &h.Score); err != nil {
			return nil, err
		}
		h.SourceRefID = strings.TrimSpace(sourceRef.String)
		h.EvidenceRefs = splitFlatRefs(refs.String)
		hits = append(hits, h)
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func (s *Service) searchKeywordFallback(ctx context.Context, query string, topK int, repoSnapshotFilters []string, sourceKinds []string) ([]Hit, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("searchdb: not initialized")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("searchdb: query required")
	}
	if len(query) >= 2 {
		if strings.HasPrefix(query, "\"") && strings.HasSuffix(query, "\"") {
			query = strings.TrimSuffix(strings.TrimPrefix(query, "\""), "\"")
		} else if strings.HasPrefix(query, "'") && strings.HasSuffix(query, "'") {
			query = strings.TrimSuffix(strings.TrimPrefix(query, "'"), "'")
		}
		query = strings.TrimSpace(query)
	}
	if query == "" {
		return nil, fmt.Errorf("searchdb: query required")
	}
	if topK <= 0 {
		topK = 10
	}
	limit := stableRecallLimit(topK)
	where, args := buildFilterWhere(repoSnapshotFilters, sourceKinds)

	sqlText := fmt.Sprintf(`
SELECT
  c.chunk_id,
  c.source_kind,
  c.title,
  c.text_excerpt,
  c.repo_source_id,
  c.repo_snapshot_id,
  c.analysis_run_id,
  c.source_ref_id,
  c.evidence_refs_flat,
  CASE
    WHEN instr(lower(c.title), lower(?)) > 0 THEN 1.0
    WHEN instr(lower(c.search_text), lower(?)) > 0 THEN 0.8
    WHEN instr(lower(c.tags_flat), lower(?)) > 0 THEN 0.6
    WHEN instr(lower(c.symbols_flat), lower(?)) > 0 THEN 0.6
    WHEN instr(lower(c.evidence_refs_flat), lower(?)) > 0 THEN 0.4
    ELSE 0.0
  END AS score
FROM kb_chunks c
WHERE (
  instr(lower(c.title), lower(?)) > 0
  OR instr(lower(c.search_text), lower(?)) > 0
  OR instr(lower(c.tags_flat), lower(?)) > 0
  OR instr(lower(c.symbols_flat), lower(?)) > 0
  OR instr(lower(c.evidence_refs_flat), lower(?)) > 0
) %s
ORDER BY score DESC, c.updated_at DESC
LIMIT ?;`, where)

	args = append([]any{query, query, query, query, query, query, query, query, query, query}, args...)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := []Hit{}
	for rows.Next() {
		var h Hit
		var refs sql.NullString
		var sourceRef sql.NullString
		if err := rows.Scan(&h.ChunkID, &h.SourceKind, &h.Title, &h.TextExcerpt, &h.RepoSourceID, &h.RepoSnapshotID, &h.AnalysisRunID, &sourceRef, &refs, &h.Score); err != nil {
			return nil, err
		}
		h.SourceRefID = strings.TrimSpace(sourceRef.String)
		h.EvidenceRefs = splitFlatRefs(refs.String)
		hits = append(hits, h)
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func (s *Service) SearchTitleMatches(ctx context.Context, query string, topK int, repoSnapshotFilters []string, sourceKinds []string) ([]Hit, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("searchdb: not initialized")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("searchdb: query required")
	}
	limit := stableRecallLimit(topK)
	where, args := buildFilterWhere(repoSnapshotFilters, sourceKinds)

	sqlText := fmt.Sprintf(`
SELECT
  c.chunk_id,
  c.source_kind,
  c.title,
  c.text_excerpt,
  c.repo_source_id,
  c.repo_snapshot_id,
  c.analysis_run_id,
  c.source_ref_id,
  c.evidence_refs_flat,
  CASE
    WHEN instr(lower(c.title), lower(?)) > 0 THEN 1.0
    WHEN instr(lower(c.search_text), lower(?)) > 0 THEN 0.72
    ELSE 0.0
  END AS score
FROM kb_chunks c
WHERE (
  instr(lower(c.title), lower(?)) > 0
  OR instr(lower(c.search_text), lower(?)) > 0
) %s
ORDER BY
  CASE WHEN instr(lower(c.title), lower(?)) > 0 THEN 0 ELSE 1 END,
  score DESC,
  length(c.title) ASC
LIMIT ?;`, where)

	args = append([]any{query, query, query, query}, args...)
	args = append(args, query, limit)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []Hit{}
	for rows.Next() {
		var h Hit
		var refs sql.NullString
		var sourceRef sql.NullString
		if err := rows.Scan(&h.ChunkID, &h.SourceKind, &h.Title, &h.TextExcerpt, &h.RepoSourceID, &h.RepoSnapshotID, &h.AnalysisRunID, &sourceRef, &refs, &h.Score); err != nil {
			return nil, err
		}
		h.SourceRefID = strings.TrimSpace(sourceRef.String)
		h.EvidenceRefs = splitFlatRefs(refs.String)
		hits = append(hits, h)
	}
	return hits, nil
}

func (s *Service) SearchVector(ctx context.Context, query string, topK int, repoSnapshotFilters []string, sourceKinds []string) ([]Hit, error) {
	if !s.VecEnabled() {
		return nil, errors.New("vector search disabled")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("searchdb: query required")
	}
	if topK <= 0 {
		topK = 10
	}
	limit := stableRecallLimit(topK)
	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	blob, err := packFloat32LE(embedding)
	if err != nil {
		return nil, err
	}
	where, args := buildVecFilterWhere(repoSnapshotFilters, sourceKinds)

	sqlText := fmt.Sprintf(`
SELECT
  c.chunk_id,
  c.source_kind,
  c.title,
  c.text_excerpt,
  c.repo_source_id,
  c.repo_snapshot_id,
  c.analysis_run_id,
  c.source_ref_id,
  c.evidence_refs_flat,
  (1.0 / (1.0 + v.distance)) AS score
FROM (
  SELECT rowid, distance
  FROM kb_chunk_vec
  WHERE embedding MATCH ? AND k = ? %s
) v
JOIN kb_chunks c ON c.rowid = v.rowid
ORDER BY v.distance ASC
LIMIT ?;`, where)

	args = append([]any{blob, limit}, args...)
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []Hit{}
	for rows.Next() {
		var h Hit
		var refs sql.NullString
		var sourceRef sql.NullString
		if err := rows.Scan(&h.ChunkID, &h.SourceKind, &h.Title, &h.TextExcerpt, &h.RepoSourceID, &h.RepoSnapshotID, &h.AnalysisRunID, &sourceRef, &refs, &h.Score); err != nil {
			return nil, err
		}
		h.SourceRefID = strings.TrimSpace(sourceRef.String)
		h.EvidenceRefs = splitFlatRefs(refs.String)
		hits = append(hits, h)
	}
	return hits, nil
}

func (s *Service) FuseAndTrim(topK int, keywordHits, vectorHits, titleHits []Hit) []Hit {
	if topK <= 0 {
		topK = 10
	}
	m := map[string]agg{}
	apply := func(h Hit, weight float64) {
		key := h.ChunkID
		existing, ok := m[key]
		if !ok {
			m[key] = agg{hit: h, score: h.Score * weight}
			return
		}
		existing.score += h.Score * weight
		// Prefer card kind titles and excerpts if present.
		if existing.hit.SourceKind != "card" && h.SourceKind == "card" {
			existing.hit = h
		}
		m[key] = existing
	}
	for _, h := range keywordHits {
		apply(h, 0.45)
	}
	for _, h := range vectorHits {
		apply(h, 0.55)
	}
	for _, h := range titleHits {
		apply(h, 0.9)
	}
	all := make([]agg, 0, len(m))
	for _, v := range m {
		v.score *= sourceKindBoost(v.hit.SourceKind)
		v.hit.Score = v.score
		all = append(all, v)
	}
	sortAgg(all)
	out := make([]Hit, 0, topK)
	for _, item := range all {
		out = append(out, item.hit)
		if len(out) >= topK {
			break
		}
	}
	return out
}

func sourceKindBoost(kind string) float64 {
	switch strings.TrimSpace(kind) {
	case "card":
		return 1.35
	case "report_section":
		return 0.92
	case "evidence":
		return 0.75
	default:
		return 1
	}
}

func stableRecallLimit(topK int) int {
	if topK <= 0 {
		return 20
	}
	if topK < 20 {
		return 20
	}
	return topK
}

func (s *Service) RebuildMeta(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("searchdb: not initialized")
	}
	meta := Meta{
		SchemaVersion:        schemaVersion,
		EmbedderModelID:      "",
		EmbedderDim:          0,
		ChunkStrategyVersion: chunkStrategyVersion,
		ScoringVersion:       scoringVersion,
		BuiltAt:              time.Now().UnixMilli(),
	}
	if s.embedder != nil {
		meta.EmbedderModelID = s.embedder.ModelID()
		meta.EmbedderDim = s.embedder.Dim()
	}
	payload, _ := json.Marshal(meta)
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO kb_meta (id, payload_json, updated_at) VALUES ('meta', ?, ?);`, string(payload), meta.BuiltAt)
	return err
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}
