package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

type startWorkflowRequest struct {
	Prompt   string `json:"prompt"`
	ExpertID string `json:"expert_id"`
}

type startWorkflowResponse struct {
	Workflow  store.Workflow      `json:"workflow"`
	Node      store.Node          `json:"master_node"`
	Execution execution.Execution `json:"execution"`
}

// startWorkflowHandler 功能：启动 workflow（MVP：创建 master node + execution，并用 runner 立即执行）。
// 参数/返回：依赖 store 与 executions manager；返回 startWorkflowResponse。
// 失败场景：workflow 不存在返回 404；冲突（已启动/已有 master）返回 409；启动或落库失败返回 5xx。
// 副作用：写入 SQLite nodes/executions/events，启动子进程并写日志文件/推送 WS。
func startWorkflowHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		if deps.Executions == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "execution manager not configured"})
			return
		}

		workflowID := c.Param("id")
		var req startWorkflowRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		wf, node, err := deps.Store.StartWorkflowMaster(c.Request.Context(), workflowID, store.StartWorkflowMasterParams{
			ExpertID: req.ExpertID,
			Prompt:   req.Prompt,
		})
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}
			if errors.Is(err, store.ErrConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		spec := masterStubSpec(wf, node)
		var exec execution.Execution
		exec, err = deps.Executions.StartOneshotWithOptions(context.Background(), spec, execution.StartOptions{
			WorkflowID: wf.ID,
			NodeID:     node.ID,
			OnExit: func(final execution.Execution) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				endedAt := time.Now().UnixMilli()
				if final.EndedAt != nil {
					endedAt = final.EndedAt.UnixMilli()
				}
				startedAt := final.StartedAt.UnixMilli()

				updatedWf, updatedNode, err := deps.Store.FinalizeExecution(ctx, store.FinalizeExecutionParams{
					ExecutionID: final.ID,
					WorkflowID:  final.WorkflowID,
					NodeID:      final.NodeID,
					Status:      string(final.Status),
					ExitCode:    final.ExitCode,
					Signal:      final.Signal,
					StartedAt:   startedAt,
					EndedAt:     endedAt,
				})
				if err != nil {
					logx.Warn("workflow", "finalize-execution", "execution 终态落库失败", "err", err, "workflow_id", final.WorkflowID, "node_id", final.NodeID, "execution_id", final.ID)
					return
				}

				broadcastWorkflowUpdated(deps.Hub, updatedWf)
				broadcastNodeUpdated(deps.Hub, updatedNode)
			},
		})
		if err != nil {
			_ = deps.Store.MarkNodeAndWorkflowFailed(context.Background(), wf.ID, node.ID, err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		logPath, err := paths.ExecutionLogPath(exec.ID)
		if err != nil {
			_ = deps.Store.MarkNodeAndWorkflowFailed(context.Background(), wf.ID, node.ID, err.Error())
			_ = deps.Executions.Cancel(exec.ID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		updatedNode, err := deps.Store.StartExecution(c.Request.Context(), store.StartExecutionParams{
			ExecutionID: exec.ID,
			WorkflowID:  wf.ID,
			NodeID:      node.ID,
			Attempt:     1,
			PID:         exec.PID,
			LogPath:     logPath,
			StartedAt:   exec.StartedAt.UnixMilli(),
			Command:     exec.Command,
			Args:        exec.Args,
			Cwd:         exec.Cwd,
		})
		if err != nil {
			_ = deps.Store.MarkNodeAndWorkflowFailed(context.Background(), wf.ID, node.ID, err.Error())
			_ = deps.Executions.Cancel(exec.ID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		broadcastWorkflowUpdated(deps.Hub, wf)
		broadcastNodeUpdated(deps.Hub, updatedNode)

		c.JSON(http.StatusOK, startWorkflowResponse{
			Workflow:  wf,
			Node:      updatedNode,
			Execution: exec,
		})
	}
}

// listWorkflowNodesHandler 功能：列出 workflow 下的所有 nodes。
// 参数/返回：依赖 store；返回 []store.Node。
// 失败场景：workflow 不存在返回 404；查询失败返回 500。
// 副作用：读取 SQLite。
func listWorkflowNodesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}

		workflowID := c.Param("id")
		if _, err := deps.Store.GetWorkflow(c.Request.Context(), workflowID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		nodes, err := deps.Store.ListNodes(c.Request.Context(), workflowID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, nodes)
	}
}

func masterStubSpec(wf store.Workflow, node store.Node) runner.RunSpec {
	cwd := wf.WorkspacePath

	return runner.RunSpec{
		Command: "bash",
		Args: []string{
			"-lc",
			`printf '\033[36m[vibe-tree]\033[0m master node started\n'; printf 'workflow_id=%s node_id=%s\n' "` + wf.ID + `" "` + node.ID + `"; for i in {1..40}; do printf '\033[32m[%03d]\033[0m planning...\n' "$i"; sleep 0.02; done; printf '\033[36m[vibe-tree]\033[0m master node finished\n'`,
		},
		Cwd: cwd,
	}
}
