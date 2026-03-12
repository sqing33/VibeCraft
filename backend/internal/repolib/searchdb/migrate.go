package searchdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

	baseStmts := []string{
		`
CREATE TABLE IF NOT EXISTS kb_meta (
  id TEXT PRIMARY KEY,
  payload_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);`,
	}
	for _, stmt := range baseStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("searchdb migrate stmt: %w", err)
		}
	}

	// Ensure kb_chunks schema matches current code expectations. This search DB is
	// purely derived data, so if schema changed we rebuild the table instead of
	// attempting a lossy/partial migration.
	if cols, err := tableColumns(ctx, tx, "kb_chunks"); err == nil && shouldRebuildChunksTable(cols) {
		_ = dropFTSTriggers(ctx, tx)
		_, _ = tx.ExecContext(ctx, `DROP TABLE IF EXISTS kb_chunks_fts;`)
		_, _ = tx.ExecContext(ctx, `DROP TABLE IF EXISTS kb_chunks;`)
	}

	chunksStmts := []string{
		`
CREATE TABLE IF NOT EXISTS kb_chunks (
  chunk_id TEXT PRIMARY KEY,
  repo_source_id TEXT NOT NULL,
  analysis_id TEXT NOT NULL,
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
		`CREATE INDEX IF NOT EXISTS idx_kb_chunks_analysis_kind ON kb_chunks(analysis_id, source_kind, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_kb_chunks_hash ON kb_chunks(content_hash);`,
	}
	for _, stmt := range chunksStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("searchdb migrate stmt: %w", err)
		}
	}

	ftsStmts := []string{
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

	ftsEnabled := true
	for _, stmt := range ftsStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			// Some environments compile SQLite without FTS5. In that case, keep the
			// DB usable via keyword fallback queries against kb_chunks.
			if strings.Contains(err.Error(), "no such module: fts5") {
				ftsEnabled = false
				// If this DB file was previously created with FTS enabled, triggers
				// might still exist and will fail every write into kb_chunks.
				_ = dropFTSTriggers(ctx, tx)
				break
			}
			return fmt.Errorf("searchdb migrate stmt: %w", err)
		}
	}
	s.ftsEnabled = ftsEnabled

	if err := tx.Commit(); err != nil {
		return err
	}
	// Ensure meta exists.
	_ = s.RebuildMeta(context.Background())
	return nil
}

func dropFTSTriggers(ctx context.Context, tx *sql.Tx) error {
	// We keep the content tables usable even when FTS5 is unavailable by
	// removing triggers that sync into kb_chunks_fts (which would hard-fail).
	for _, name := range []string{"kb_chunks_ai", "kb_chunks_ad", "kb_chunks_au"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s;`, name)); err != nil {
			return err
		}
	}
	return nil
}

type tableInfoQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func tableColumns(ctx context.Context, q tableInfoQueryer, tableName string) ([]string, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s);`, strings.TrimSpace(tableName)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		out = append(out, strings.TrimSpace(name))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func shouldRebuildChunksTable(cols []string) bool {
	if len(cols) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, c := range cols {
		set[strings.TrimSpace(c)] = struct{}{}
	}
	if _, ok := set["analysis_id"]; !ok {
		return true
	}
	if _, ok := set["repo_snapshot_id"]; ok {
		return true
	}
	if _, ok := set["analysis_run_id"]; ok {
		return true
	}
	return false
}

func pingWithTimeout(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return db.PingContext(pctx)
}
