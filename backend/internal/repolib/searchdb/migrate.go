package searchdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Service) applyPragmas(ctx context.Context) error {
	stmts := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("searchdb pragma %q: %w", stmt, err)
		}
	}
	return nil
}

func (s *Service) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS kb_meta (
  id TEXT PRIMARY KEY,
  payload_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);`,
		`
CREATE TABLE IF NOT EXISTS kb_chunks (
  chunk_id TEXT PRIMARY KEY,
  repo_source_id TEXT NOT NULL,
  repo_snapshot_id TEXT NOT NULL,
  analysis_run_id TEXT NOT NULL,
  source_kind TEXT NOT NULL,
  source_ref_id TEXT,
  title TEXT,
  display_text TEXT NOT NULL,
  search_text TEXT NOT NULL,
  tags_flat TEXT,
  symbols_flat TEXT,
  evidence_refs_flat TEXT,
  text_excerpt TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_kb_chunks_snapshot_kind ON kb_chunks(repo_snapshot_id, source_kind, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_kb_chunks_hash ON kb_chunks(content_hash);`,
		`
CREATE VIRTUAL TABLE IF NOT EXISTS kb_chunks_fts USING fts5(
  title,
  tags_flat,
  symbols_flat,
  evidence_refs_flat,
  search_text,
  content='kb_chunks',
  content_rowid='rowid'
);`,
		`
CREATE TRIGGER IF NOT EXISTS kb_chunks_ai AFTER INSERT ON kb_chunks BEGIN
  INSERT INTO kb_chunks_fts(rowid, title, tags_flat, symbols_flat, evidence_refs_flat, search_text)
  VALUES (new.rowid, new.title, new.tags_flat, new.symbols_flat, new.evidence_refs_flat, new.search_text);
END;`,
		`
CREATE TRIGGER IF NOT EXISTS kb_chunks_ad AFTER DELETE ON kb_chunks BEGIN
  INSERT INTO kb_chunks_fts(kb_chunks_fts, rowid, title, tags_flat, symbols_flat, evidence_refs_flat, search_text)
  VALUES ('delete', old.rowid, old.title, old.tags_flat, old.symbols_flat, old.evidence_refs_flat, old.search_text);
END;`,
		`
CREATE TRIGGER IF NOT EXISTS kb_chunks_au AFTER UPDATE ON kb_chunks BEGIN
  INSERT INTO kb_chunks_fts(kb_chunks_fts, rowid, title, tags_flat, symbols_flat, evidence_refs_flat, search_text)
  VALUES ('delete', old.rowid, old.title, old.tags_flat, old.symbols_flat, old.evidence_refs_flat, old.search_text);
  INSERT INTO kb_chunks_fts(rowid, title, tags_flat, symbols_flat, evidence_refs_flat, search_text)
  VALUES (new.rowid, new.title, new.tags_flat, new.symbols_flat, new.evidence_refs_flat, new.search_text);
END;`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("searchdb migrate stmt: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Ensure meta exists.
	_ = s.RebuildMeta(context.Background())
	return nil
}

func pingWithTimeout(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return db.PingContext(pctx)
}
