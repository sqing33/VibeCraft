package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"vibecraft/backend/internal/dag"
	"vibecraft/backend/internal/id"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()

	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func TestStateMachine_Workflow_MasterOnly_SucceedsAndRejectsCancel(t *testing.T) {
	st := openTestStore(t)

	wf, err := st.CreateWorkflow(context.Background(), CreateWorkflowParams{
		Title:         "master-only",
		WorkspacePath: ".",
		Mode:          "manual",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	startedWf, master, err := st.StartWorkflowMaster(context.Background(), wf.ID, StartWorkflowMasterParams{})
	if err != nil {
		t.Fatalf("start master: %v", err)
	}
	if startedWf.Status != string(WorkflowStatusRunning) {
		t.Fatalf("expected workflow running, got %q", startedWf.Status)
	}

	execID := id.New("ex_")
	now := time.Now().UnixMilli()
	logPath := filepath.Join(t.TempDir(), execID+".log")

	if _, err := st.StartExecution(context.Background(), StartExecutionParams{
		ExecutionID: execID,
		WorkflowID:  wf.ID,
		NodeID:      master.ID,
		Attempt:     1,
		PID:         123,
		LogPath:     logPath,
		StartedAt:   now,
		Command:     "bash",
		Args:        []string{"-lc", "echo ok"},
		Cwd:         ".",
	}); err != nil {
		t.Fatalf("start execution: %v", err)
	}

	finalWf, _, err := st.FinalizeExecution(context.Background(), FinalizeExecutionParams{
		ExecutionID: execID,
		WorkflowID:  wf.ID,
		NodeID:      master.ID,
		Status:      "succeeded",
		ExitCode:    0,
		StartedAt:   now,
		EndedAt:     now + 10,
	})
	if err != nil {
		t.Fatalf("finalize execution: %v", err)
	}
	if finalWf.Status != string(WorkflowStatusDone) {
		t.Fatalf("expected workflow done, got %q", finalWf.Status)
	}

	if _, err := st.CancelWorkflow(context.Background(), wf.ID); err == nil {
		t.Fatalf("expected cancel conflict error")
	} else if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestStateMachine_NodeEditability_RejectsRunningAndSucceeded(t *testing.T) {
	st := openTestStore(t)

	wf, err := st.CreateWorkflow(context.Background(), CreateWorkflowParams{
		Title:         "editability",
		WorkspacePath: ".",
		Mode:          "manual",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if _, _, err := st.StartWorkflowMaster(context.Background(), wf.ID, StartWorkflowMasterParams{}); err != nil {
		t.Fatalf("start master: %v", err)
	}

	_, createdNodes, _, err := st.ApplyDAG(context.Background(), wf.ID, dag.DAG{
		Nodes: []dag.DAGNode{
			{ID: "a", Title: "Alpha", ExpertID: "bash", Prompt: "echo alpha"},
		},
	})
	if err != nil {
		t.Fatalf("apply dag: %v", err)
	}
	if len(createdNodes) != 1 {
		t.Fatalf("expected 1 created node, got %d", len(createdNodes))
	}
	worker := createdNodes[0]

	newPrompt := "echo changed"
	if _, err := st.UpdateNode(context.Background(), worker.ID, UpdateNodeParams{Prompt: &newPrompt}); err != nil {
		t.Fatalf("update node in pending_approval should succeed, got %v", err)
	}

	if _, approved, err := st.ApproveRunnableNodes(context.Background(), wf.ID); err != nil {
		t.Fatalf("approve runnable: %v", err)
	} else if len(approved) != 1 || approved[0].ID != worker.ID || approved[0].Status != "queued" {
		t.Fatalf("expected worker queued after approve, got %#v", approved)
	}

	execID := id.New("ex_")
	now := time.Now().UnixMilli()
	logPath := filepath.Join(t.TempDir(), execID+".log")

	if _, err := st.StartExecution(context.Background(), StartExecutionParams{
		ExecutionID: execID,
		WorkflowID:  wf.ID,
		NodeID:      worker.ID,
		Attempt:     1,
		PID:         123,
		LogPath:     logPath,
		StartedAt:   now,
		Command:     "bash",
		Args:        []string{"-lc", "echo running"},
		Cwd:         ".",
	}); err != nil {
		t.Fatalf("start execution: %v", err)
	}

	if _, err := st.UpdateNode(context.Background(), worker.ID, UpdateNodeParams{Prompt: &newPrompt}); err == nil {
		t.Fatalf("expected running node not editable")
	} else if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	if _, _, err := st.FinalizeExecution(context.Background(), FinalizeExecutionParams{
		ExecutionID: execID,
		WorkflowID:  wf.ID,
		NodeID:      worker.ID,
		Status:      "succeeded",
		ExitCode:    0,
		StartedAt:   now,
		EndedAt:     now + 10,
	}); err != nil {
		t.Fatalf("finalize execution: %v", err)
	}

	if _, err := st.UpdateNode(context.Background(), worker.ID, UpdateNodeParams{Prompt: &newPrompt}); err == nil {
		t.Fatalf("expected succeeded node not editable")
	} else if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	if _, _, err := st.RetryNode(context.Background(), worker.ID); err == nil {
		t.Fatalf("expected succeeded node not retryable")
	} else if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestStateMachine_FailFastAndRetry_UnskipsNodes(t *testing.T) {
	st := openTestStore(t)

	wf, err := st.CreateWorkflow(context.Background(), CreateWorkflowParams{
		Title:         "fail-fast",
		WorkspacePath: ".",
		Mode:          "manual",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if _, _, err := st.StartWorkflowMaster(context.Background(), wf.ID, StartWorkflowMasterParams{}); err != nil {
		t.Fatalf("start master: %v", err)
	}

	_, createdNodes, _, err := st.ApplyDAG(context.Background(), wf.ID, dag.DAG{
		Nodes: []dag.DAGNode{
			{ID: "a", Title: "Alpha", ExpertID: "bash", Prompt: "echo alpha"},
			{ID: "b", Title: "Beta", ExpertID: "bash", Prompt: "echo beta"},
		},
		Edges: []dag.DAGEdge{
			{From: "a", To: "b", Type: "success"},
		},
	})
	if err != nil {
		t.Fatalf("apply dag: %v", err)
	}
	if len(createdNodes) != 2 {
		t.Fatalf("expected 2 created nodes, got %d", len(createdNodes))
	}
	alpha := createdNodes[0]
	beta := createdNodes[1]

	if _, approved, err := st.ApproveRunnableNodes(context.Background(), wf.ID); err != nil {
		t.Fatalf("approve runnable: %v", err)
	} else if len(approved) != 1 || approved[0].ID != alpha.ID || approved[0].Status != "queued" {
		t.Fatalf("expected only alpha queued after approve, got %#v", approved)
	}

	execID := id.New("ex_")
	now := time.Now().UnixMilli()
	logPath := filepath.Join(t.TempDir(), execID+".log")
	if _, err := st.StartExecution(context.Background(), StartExecutionParams{
		ExecutionID: execID,
		WorkflowID:  wf.ID,
		NodeID:      alpha.ID,
		Attempt:     1,
		PID:         123,
		LogPath:     logPath,
		StartedAt:   now,
		Command:     "bash",
		Args:        []string{"-lc", "exit 1"},
		Cwd:         ".",
	}); err != nil {
		t.Fatalf("start execution: %v", err)
	}

	if _, _, err := st.FinalizeExecution(context.Background(), FinalizeExecutionParams{
		ExecutionID:  execID,
		WorkflowID:   wf.ID,
		NodeID:       alpha.ID,
		Status:       "failed",
		ExitCode:     1,
		StartedAt:    now,
		EndedAt:      now + 10,
		ErrorMessage: "exit_code=1",
	}); err != nil {
		t.Fatalf("finalize execution: %v", err)
	}

	latestWf, err := st.GetWorkflow(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	if latestWf.Status != string(WorkflowStatusFailed) {
		t.Fatalf("expected workflow failed after worker failure, got %q", latestWf.Status)
	}

	allNodes, err := st.ListNodes(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	var gotAlpha, gotBeta *Node
	for i := range allNodes {
		switch allNodes[i].ID {
		case alpha.ID:
			gotAlpha = &allNodes[i]
		case beta.ID:
			gotBeta = &allNodes[i]
		}
	}
	if gotAlpha == nil || gotBeta == nil {
		t.Fatalf("missing nodes after finalize")
	}
	if gotBeta.Status != "skipped" {
		t.Fatalf("expected beta skipped, got %q", gotBeta.Status)
	}
	if gotBeta.ErrorMessage == nil || *gotBeta.ErrorMessage != "fail_fast" {
		t.Fatalf("expected beta error_message=fail_fast, got %#v", gotBeta.ErrorMessage)
	}

	updatedWf, updatedNodes, err := st.RetryNode(context.Background(), alpha.ID)
	if err != nil {
		t.Fatalf("retry node: %v", err)
	}
	if updatedWf.Status != string(WorkflowStatusRunning) {
		t.Fatalf("expected workflow running after retry, got %q", updatedWf.Status)
	}

	var retriedAlpha, unskippedBeta *Node
	for i := range updatedNodes {
		switch updatedNodes[i].ID {
		case alpha.ID:
			retriedAlpha = &updatedNodes[i]
		case beta.ID:
			unskippedBeta = &updatedNodes[i]
		}
	}
	if retriedAlpha == nil || unskippedBeta == nil {
		t.Fatalf("expected retry to update alpha and unskip beta, got %#v", updatedNodes)
	}
	if retriedAlpha.Status != "queued" {
		t.Fatalf("expected retried alpha queued, got %q", retriedAlpha.Status)
	}
	if unskippedBeta.Status != "pending_approval" {
		t.Fatalf("expected unskipped beta pending_approval, got %q", unskippedBeta.Status)
	}
	if unskippedBeta.ErrorMessage != nil {
		t.Fatalf("expected unskipped beta error_message cleared, got %#v", unskippedBeta.ErrorMessage)
	}
}

func TestStateMachine_WorkflowModeSwitch_QueuedToPendingApprovalAndBack(t *testing.T) {
	st := openTestStore(t)

	wf, err := st.CreateWorkflow(context.Background(), CreateWorkflowParams{
		Title:         "mode-switch",
		WorkspacePath: ".",
		Mode:          "auto",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if _, _, err := st.StartWorkflowMaster(context.Background(), wf.ID, StartWorkflowMasterParams{}); err != nil {
		t.Fatalf("start master: %v", err)
	}

	_, createdNodes, _, err := st.ApplyDAG(context.Background(), wf.ID, dag.DAG{
		Nodes: []dag.DAGNode{
			{ID: "a", Title: "Alpha", ExpertID: "bash", Prompt: "echo alpha"},
			{ID: "b", Title: "Beta", ExpertID: "bash", Prompt: "echo beta"},
		},
	})
	if err != nil {
		t.Fatalf("apply dag: %v", err)
	}
	for _, n := range createdNodes {
		if n.Status != "queued" {
			t.Fatalf("expected auto-mode nodes queued, got %q for node %s", n.Status, n.ID)
		}
	}

	modeManual := "manual"
	if _, switched, err := st.UpdateWorkflow(context.Background(), wf.ID, UpdateWorkflowParams{Mode: &modeManual}); err != nil {
		t.Fatalf("switch to manual: %v", err)
	} else if len(switched) != 2 {
		t.Fatalf("expected 2 switched nodes, got %d", len(switched))
	} else {
		for _, n := range switched {
			if n.Status != "pending_approval" {
				t.Fatalf("expected node pending_approval after switch to manual, got %q", n.Status)
			}
		}
	}

	modeAuto := "auto"
	if _, switched, err := st.UpdateWorkflow(context.Background(), wf.ID, UpdateWorkflowParams{Mode: &modeAuto}); err != nil {
		t.Fatalf("switch to auto: %v", err)
	} else if len(switched) != 2 {
		t.Fatalf("expected 2 switched nodes back, got %d", len(switched))
	} else {
		for _, n := range switched {
			if n.Status != "queued" {
				t.Fatalf("expected node queued after switch back to auto, got %q", n.Status)
			}
		}
	}
}

func TestStateMachine_CancelWorkflow_CancelsQueuedNodesAndReturnsRunningExecutions(t *testing.T) {
	st := openTestStore(t)

	wf, err := st.CreateWorkflow(context.Background(), CreateWorkflowParams{
		Title:         "cancel",
		WorkspacePath: ".",
		Mode:          "manual",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if _, _, err := st.StartWorkflowMaster(context.Background(), wf.ID, StartWorkflowMasterParams{}); err != nil {
		t.Fatalf("start master: %v", err)
	}

	_, createdNodes, _, err := st.ApplyDAG(context.Background(), wf.ID, dag.DAG{
		Nodes: []dag.DAGNode{
			{ID: "a", Title: "Alpha", ExpertID: "bash", Prompt: "echo alpha"},
			{ID: "b", Title: "Beta", ExpertID: "bash", Prompt: "echo beta"},
		},
	})
	if err != nil {
		t.Fatalf("apply dag: %v", err)
	}
	alpha := createdNodes[0]
	beta := createdNodes[1]

	if _, approved, err := st.ApproveRunnableNodes(context.Background(), wf.ID); err != nil {
		t.Fatalf("approve runnable: %v", err)
	} else if len(approved) != 2 {
		t.Fatalf("expected 2 approved nodes, got %d", len(approved))
	}

	execID := id.New("ex_")
	now := time.Now().UnixMilli()
	logPath := filepath.Join(t.TempDir(), execID+".log")
	if _, err := st.StartExecution(context.Background(), StartExecutionParams{
		ExecutionID: execID,
		WorkflowID:  wf.ID,
		NodeID:      alpha.ID,
		Attempt:     1,
		PID:         123,
		LogPath:     logPath,
		StartedAt:   now,
		Command:     "bash",
		Args:        []string{"-lc", "sleep 10"},
		Cwd:         ".",
	}); err != nil {
		t.Fatalf("start execution: %v", err)
	}

	res, err := st.CancelWorkflow(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("cancel workflow: %v", err)
	}
	if res.Workflow.Status != string(WorkflowStatusCanceled) {
		t.Fatalf("expected workflow canceled, got %q", res.Workflow.Status)
	}
	if len(res.RunningExecutionIDs) != 1 || res.RunningExecutionIDs[0] != execID {
		t.Fatalf("expected running execution ids to include %s, got %#v", execID, res.RunningExecutionIDs)
	}

	foundBeta := false
	for _, n := range res.CanceledNodes {
		if n.ID != beta.ID {
			continue
		}
		foundBeta = true
		if n.Status != "canceled" {
			t.Fatalf("expected beta canceled, got %q", n.Status)
		}
		if n.ErrorMessage == nil || *n.ErrorMessage != "workflow_canceled" {
			t.Fatalf("expected beta error_message=workflow_canceled, got %#v", n.ErrorMessage)
		}
	}
	if !foundBeta {
		t.Fatalf("expected cancel to mark beta canceled")
	}
}
