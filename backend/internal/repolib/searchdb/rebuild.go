package searchdb

import (
	"context"
	"fmt"
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
