package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/dag"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
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
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
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

		startExpertID := strings.TrimSpace(req.ExpertID)
		if startExpertID == "" {
			startExpertID = "master"
		}

		startPrompt := strings.TrimSpace(req.Prompt)
		if startPrompt == "" && startExpertID != "bash" {
			startPrompt = defaultMasterPromptTemplate(workflowID, deps.Experts.KnownIDs())
		}

		wf, node, err := deps.Store.StartWorkflowMaster(c.Request.Context(), workflowID, store.StartWorkflowMasterParams{
			ExpertID: startExpertID,
			Prompt:   startPrompt,
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

		resolved, err := deps.Experts.Resolve(node.ExpertID, node.Prompt, wf.WorkspacePath)
		if err != nil {
			_ = deps.Store.MarkNodeAndWorkflowFailed(context.Background(), wf.ID, node.ID, err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		spec := resolved.Spec
		execCtx := context.Background()
		cancelExec := func() {}
		if resolved.Timeout > 0 {
			execCtx, cancelExec = context.WithTimeout(context.Background(), resolved.Timeout)
		}

		var exec execution.Execution
		exec, err = deps.Executions.StartOneshotWithOptions(execCtx, spec, execution.StartOptions{
			WorkflowID: wf.ID,
			NodeID:     node.ID,
			OnExit: func(final execution.Execution) {
				cancelExec()

				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				finalStatus := string(final.Status)
				finalError := ""
				var finalSummary *string

				// Master 成功退出后尝试解析 DAG 并落库；解析/落库失败视为 workflow 失败（MVP）。
				if final.Status == execution.StatusSucceeded {
					logPath, err := paths.ExecutionLogPath(final.ID)
					if err != nil {
						finalStatus = "failed"
						finalError = err.Error()
						finalSummary = &finalError
					} else if b, err := os.ReadFile(logPath); err != nil {
						finalStatus = "failed"
						finalError = fmt.Sprintf("read master output: %v", err)
						finalSummary = &finalError
					} else if d, err := dag.ParseAndValidate(b, dag.ValidateOptions{
						KnownExperts: deps.Experts.KnownIDs(),
					}); err != nil {
						finalStatus = "failed"
						finalError = fmt.Sprintf("invalid dag: %v", err)
						s := fmt.Sprintf("%s\n\noutput_tail:\n%s", finalError, summarizeOutputTail(b))
						finalSummary = &s
					} else if updatedWf, createdNodes, createdEdges, err := deps.Store.ApplyDAG(ctx, final.WorkflowID, d); err != nil {
						finalStatus = "failed"
						finalError = fmt.Sprintf("apply dag: %v", err)
						s := fmt.Sprintf("%s\n\noutput_tail:\n%s", finalError, summarizeOutputTail(b))
						finalSummary = &s
					} else {
						broadcastWorkflowUpdated(deps.Hub, updatedWf)
						for _, n := range createdNodes {
							broadcastNodeUpdated(deps.Hub, n)
						}
						broadcastDAGGenerated(deps.Hub, updatedWf.ID, len(createdNodes), len(createdEdges))
					}
				}

				endedAt := time.Now().UnixMilli()
				if final.EndedAt != nil {
					endedAt = final.EndedAt.UnixMilli()
				}
				startedAt := final.StartedAt.UnixMilli()

				if finalStatus == "failed" && finalError == "" {
					if final.Signal != "" {
						finalError = fmt.Sprintf("signal=%s exit_code=%d", final.Signal, final.ExitCode)
					} else {
						finalError = fmt.Sprintf("exit_code=%d", final.ExitCode)
					}
				} else if finalStatus == "canceled" && finalError == "" {
					finalError = "canceled"
				} else if finalStatus == "timeout" && finalError == "" {
					finalError = "timeout"
				}
				if (finalStatus == "failed" || finalStatus == "timeout") && finalSummary == nil {
					if b, err := execution.ReadLogTail(final.ID, 4000); err == nil && len(b) > 0 {
						s := string(b)
						finalSummary = &s
					}
				}

				updatedWf, updatedNodes, err := deps.Store.FinalizeExecution(ctx, store.FinalizeExecutionParams{
					ExecutionID:   final.ID,
					WorkflowID:    final.WorkflowID,
					NodeID:        final.NodeID,
					Status:        finalStatus,
					ExitCode:      final.ExitCode,
					Signal:        final.Signal,
					StartedAt:     startedAt,
					EndedAt:       endedAt,
					ErrorMessage:  finalError,
					ResultSummary: finalSummary,
				})
				if err != nil {
					logx.Warn("workflow", "finalize-execution", "execution 终态落库失败", "err", err, "workflow_id", final.WorkflowID, "node_id", final.NodeID, "execution_id", final.ID)
					return
				}

				broadcastWorkflowUpdated(deps.Hub, updatedWf)
				for _, n := range updatedNodes {
					broadcastNodeUpdated(deps.Hub, n)
				}
			},
		})
		if err != nil {
			cancelExec()
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
			Attempt:     0,
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

func summarizeOutputTail(b []byte) string {
	const max = 4000
	if len(b) <= max {
		return string(b)
	}
	return string(b[len(b)-max:])
}

func defaultMasterPromptTemplate(workflowID string, knownExperts map[string]struct{}) string {
	expertIDs := make([]string, 0, len(knownExperts))
	for id := range knownExperts {
		expertIDs = append(expertIDs, id)
	}
	sort.Strings(expertIDs)
	known := strings.Join(expertIDs, ", ")

	return fmt.Sprintf(`You are the workflow master planner for vibe-tree.

Output MUST be a single JSON object (no markdown, no extra text) that follows this shape:
{
  "workflow_title": "",
  "nodes": [
    {
      "id": "n1",
      "title": "Step 1",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "",
      "prompt": "echo hello"
    },
    {
      "id": "n2",
      "title": "Step 2",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "",
      "prompt": "echo world"
    }
  ],
  "edges": [
    { "from": "n1", "to": "n2", "type": "success" }
  ]
}

Constraints:
- nodes[].id must be unique.
- edges[].from/to must reference existing nodes.
- edge.type should be "success".
- expert_id MUST be one of configured expert_id values: %s

workflow_id=%s`, known, workflowID)
}
