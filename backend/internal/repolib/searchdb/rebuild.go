package searchdb

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) DeleteAll(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("searchdb: not initialized")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM kb_chunks;`); err != nil {
		return err
	}
	return nil
}

func (s *Service) DeleteRun(ctx context.Context, runID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("searchdb: not initialized")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("searchdb: run_id required")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM kb_chunks WHERE analysis_run_id = ?;`, runID)
	return err
}
