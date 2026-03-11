package searchdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-sqlite3"
)

func (s *Service) tryLoadVecExtension(ctx context.Context) error {
	if s.embedder == nil || s.embedder.Dim() <= 0 {
		return fmt.Errorf("embedder not configured")
	}

	path, err := s.ensureVecExtension(ctx)
	if err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	if _, err := os.Stat(path); err != nil {
		return err
	}
	// Load extension via driver connection so we can enable/disable load_extension safely.
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	if err := conn.Raw(func(dc any) error {
		raw, ok := dc.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected driver conn type")
		}
		// sqlite-vec exports sqlite3_vec_init (not sqlite3_vec0_init).
		return raw.LoadExtension(path, "sqlite3_vec_init")
	}); err != nil {
		_ = conn.Close()
		return err
	}

	if err := ensureVecTable(ctx, conn, s.embedder.Dim()); err != nil {
		_ = conn.Close()
		return err
	}
	// Return the reserved connection back to the pool before running any other
	// db.ExecContext calls (searchdb uses max open conns = 1).
	_ = conn.Close()
	s.vecEnabled = true
	_ = s.RebuildMeta(ctx)
	return nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func ensureVecTable(ctx context.Context, exec execer, dim int) error {
	if dim <= 0 {
		return fmt.Errorf("invalid embedding dim")
	}
	// vec0 virtual table is provided by sqlite-vec extension.
	_, err := exec.ExecContext(ctx, fmt.Sprintf(`
CREATE VIRTUAL TABLE IF NOT EXISTS kb_chunk_vec USING vec0(
  embedding float[%d],
  repo_snapshot_id TEXT,
  source_kind TEXT
);
`, dim))
	return err
}
