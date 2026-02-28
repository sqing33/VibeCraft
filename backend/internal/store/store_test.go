package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartWorkflowMaster_RejectsSecondStart(t *testing.T) {
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	wf, err := st.CreateWorkflow(context.Background(), CreateWorkflowParams{
		Title:         "hello",
		WorkspacePath: ".",
		Mode:          "manual",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	if _, _, err := st.StartWorkflowMaster(context.Background(), wf.ID, StartWorkflowMasterParams{}); err != nil {
		t.Fatalf("start master: %v", err)
	}

	if _, _, err := st.StartWorkflowMaster(context.Background(), wf.ID, StartWorkflowMasterParams{}); err == nil {
		t.Fatalf("expected conflict error")
	} else if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestApproveRunnableNodes_RejectsAutoMode(t *testing.T) {
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	wf, err := st.CreateWorkflow(context.Background(), CreateWorkflowParams{
		Title:         "hello",
		WorkspacePath: ".",
		Mode:          "auto",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	if _, _, err := st.ApproveRunnableNodes(context.Background(), wf.ID); err == nil {
		t.Fatalf("expected validation error")
	} else if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestStore_ConcurrentWrites_NoDatabaseLocked(t *testing.T) {
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const goroutines = 16
	const loops = 20

	errCh := make(chan error, goroutines*loops)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < loops; i++ {
				wf, err := st.CreateWorkflow(ctx, CreateWorkflowParams{
					Title:         fmt.Sprintf("w-%d-%d", g, i),
					WorkspacePath: ".",
					Mode:          "manual",
				})
				if err != nil {
					errCh <- err
					return
				}

				newTitle := fmt.Sprintf("w2-%d-%d", g, i)
				if _, _, err := st.UpdateWorkflow(ctx, wf.ID, UpdateWorkflowParams{Title: &newTitle}); err != nil {
					errCh <- err
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err == nil {
			continue
		}
		if strings.Contains(strings.ToLower(err.Error()), "database is locked") {
			t.Fatalf("unexpected sqlite lock error: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}
}
