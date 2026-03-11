package searchdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func upsertChunkRow(ctx context.Context, tx *sql.Tx, chunk Chunk) (int64, error) {
	if chunk.UpdatedAt == 0 {
		chunk.UpdatedAt = time.Now().UnixMilli()
	}
	row := tx.QueryRowContext(
		ctx,
		`
INSERT INTO kb_chunks (
  chunk_id, repo_source_id, repo_snapshot_id, analysis_run_id,
  source_kind, source_ref_id, title,
  display_text, search_text,
  tags_flat, symbols_flat, evidence_refs_flat,
  text_excerpt, content_hash, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chunk_id) DO UPDATE SET
  repo_source_id = excluded.repo_source_id,
  repo_snapshot_id = excluded.repo_snapshot_id,
  analysis_run_id = excluded.analysis_run_id,
  source_kind = excluded.source_kind,
  source_ref_id = excluded.source_ref_id,
  title = excluded.title,
  display_text = excluded.display_text,
  search_text = excluded.search_text,
  tags_flat = excluded.tags_flat,
  symbols_flat = excluded.symbols_flat,
  evidence_refs_flat = excluded.evidence_refs_flat,
  text_excerpt = excluded.text_excerpt,
  content_hash = excluded.content_hash,
  updated_at = excluded.updated_at
RETURNING rowid;`,
		strings.TrimSpace(chunk.ChunkID),
		strings.TrimSpace(chunk.RepoSourceID),
		strings.TrimSpace(chunk.RepoSnapshotID),
		strings.TrimSpace(chunk.AnalysisRunID),
		strings.TrimSpace(chunk.SourceKind),
		trimOrNil(chunk.SourceRefID),
		trimOrNil(chunk.Title),
		strings.TrimSpace(chunk.DisplayText),
		strings.TrimSpace(chunk.SearchText),
		trimOrNil(chunk.TagsFlat),
		trimOrNil(chunk.SymbolsFlat),
		trimOrNil(chunk.EvidenceRefs),
		strings.TrimSpace(chunk.TextExcerpt),
		strings.TrimSpace(chunk.ContentHash),
		chunk.UpdatedAt,
	)
	var rowid int64
	if err := row.Scan(&rowid); err != nil {
		return 0, err
	}
	return rowid, nil
}

func upsertFTSRow(ctx context.Context, tx *sql.Tx, rowid int64, chunk Chunk) error {
	_ = ctx
	_ = tx
	_ = rowid
	_ = chunk
	// External-content FTS is kept in sync via triggers on kb_chunks.
	return nil
}

func upsertVecRow(ctx context.Context, tx *sql.Tx, rowid int64, snapshotID, sourceKind string, embedding []float32) error {
	blob, err := packFloat32LE(embedding)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO kb_chunk_vec(rowid, embedding, repo_snapshot_id, source_kind) VALUES (?, ?, ?, ?);`, rowid, blob, strings.TrimSpace(snapshotID), strings.TrimSpace(sourceKind)); err != nil {
		return fmt.Errorf("upsert vec row: %w", err)
	}
	return nil
}

func trimOrNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
