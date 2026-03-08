package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/orchestration"
	"vibe-tree/backend/internal/repolib"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

type Deps struct {
	Executions    *execution.Manager
	Hub           *ws.Hub
	Store         *store.Store
	Experts       *expert.Registry
	Chat          *chat.Manager
	Orchestration *orchestration.Manager
	RepoLibrary   *repolib.Service
}

// Register 功能：注册 HTTP/WS 路由到 `/api/v1` 路由组。
// 参数/返回：v1 为 gin RouterGroup；deps 注入执行管理器与 WS Hub；无返回值。
// 失败场景：无（依赖缺失会在具体 handler 内返回 500）。
// 副作用：向路由器挂载 handler。
func Register(v1 *gin.RouterGroup, deps Deps) {
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	v1.GET("/info", infoHandler(deps))
	v1.GET("/experts", listExpertsHandler(deps))
	v1.GET("/settings/basic", getBasicSettingsHandler())
	v1.PUT("/settings/basic", putBasicSettingsHandler())
	v1.GET("/settings/cli-tools", getCLIToolSettingsHandler())
	v1.PUT("/settings/cli-tools", putCLIToolSettingsHandler(deps))
	v1.GET("/settings/llm", getLLMSettingsHandler())
	v1.PUT("/settings/llm", putLLMSettingsHandler(deps))
	v1.POST("/settings/llm/test", llmTestHandler())
	v1.GET("/settings/experts", getExpertSettingsHandler(deps))
	v1.PUT("/settings/experts", putExpertSettingsHandler(deps))
	v1.POST("/settings/experts/generate", generateExpertSettingsHandler(deps))
	v1.GET("/settings/experts/sessions", listExpertBuilderSessionsHandler(deps))
	v1.POST("/settings/experts/sessions", createExpertBuilderSessionHandler(deps))
	v1.GET("/settings/experts/sessions/:id", getExpertBuilderSessionHandler(deps))
	v1.POST("/settings/experts/sessions/:id/messages", postExpertBuilderMessageHandler(deps))
	v1.POST("/settings/experts/sessions/:id/publish", publishExpertBuilderSnapshotHandler(deps))

	v1.POST("/chat/sessions", createChatSessionHandler(deps))
	v1.GET("/chat/sessions", listChatSessionsHandler(deps))
	v1.GET("/chat/sessions/:id/messages", listChatMessagesHandler(deps))
	v1.GET("/chat/sessions/:id/turns", listChatTurnsHandler(deps))
	v1.GET("/chat/sessions/:id/attachments/:attachmentID/content", getChatAttachmentContentHandler(deps))
	v1.PATCH("/chat/sessions/:id", patchChatSessionHandler(deps))
	v1.POST("/chat/sessions/:id/turns", postChatTurnHandler(deps))
	v1.POST("/chat/sessions/:id/compact", compactChatSessionHandler(deps))
	v1.POST("/chat/sessions/:id/fork", forkChatSessionHandler(deps))

	v1.GET("/orchestrations", listOrchestrationsHandler(deps))
	v1.POST("/orchestrations", createOrchestrationHandler(deps))
	v1.GET("/orchestrations/:id", getOrchestrationHandler(deps))
	v1.POST("/orchestrations/:id/cancel", cancelOrchestrationHandler(deps))
	v1.POST("/orchestrations/:id/continue", continueOrchestrationHandler(deps))
	v1.POST("/agent-runs/:id/retry", retryAgentRunHandler(deps))

	v1.POST("/repo-library/analyses", createRepoAnalysisHandler(deps))
	v1.POST("/repo-library/analyses/:id/sync-chat", syncRepoAnalysisChatHandler(deps))
	v1.GET("/repo-library/repositories", listRepoRepositoriesHandler(deps))
	v1.GET("/repo-library/repositories/:id", getRepoRepositoryHandler(deps))
	v1.GET("/repo-library/repositories/:id/snapshots", listRepoSnapshotsHandler(deps))
	v1.GET("/repo-library/snapshots/:id/report", getRepoSnapshotReportHandler(deps))
	v1.GET("/repo-library/cards", listRepoCardsHandler(deps))
	v1.GET("/repo-library/cards/:id", getRepoCardHandler(deps))
	v1.GET("/repo-library/cards/:id/evidence", listRepoCardEvidenceHandler(deps))
	v1.POST("/repo-library/search", searchRepoLibraryHandler(deps))

	v1.GET("/workflows", listWorkflowsHandler(deps))
	v1.POST("/workflows", createWorkflowHandler(deps))
	v1.GET("/workflows/:id", getWorkflowHandler(deps))
	v1.PATCH("/workflows/:id", patchWorkflowHandler(deps))
	v1.POST("/workflows/:id/start", startWorkflowHandler(deps))
	v1.GET("/workflows/:id/nodes", listWorkflowNodesHandler(deps))
	v1.GET("/workflows/:id/edges", listWorkflowEdgesHandler(deps))
	v1.POST("/workflows/:id/approve", approveWorkflowHandler(deps))
	v1.POST("/workflows/:id/cancel", cancelWorkflowHandler(deps))

	v1.PATCH("/nodes/:id", patchNodeHandler(deps))
	v1.POST("/nodes/:id/retry", retryNodeHandler(deps))
	v1.POST("/nodes/:id/cancel", cancelNodeHandler(deps))

	v1.POST("/executions", startExecutionHandler(deps))
	v1.POST("/executions/:id/cancel", cancelExecutionHandler(deps))
	v1.GET("/executions/:id/log", executionLogTailHandler())
	v1.GET("/ws", wsHandler(deps))
}

type startExecutionRequest struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Cwd     string            `json:"cwd"`
}

// startExecutionHandler 功能：启动一个 execution 并开始日志落盘/WS 推送。
// 参数/返回：依赖 executions manager；返回 gin.HandlerFunc。
// 失败场景：manager 未配置、请求 JSON 非法或进程启动失败时返回 4xx/5xx。
// 副作用：创建日志文件、启动子进程、推送 WS 事件。
func startExecutionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Executions == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "execution manager not configured"})
			return
		}

		var req startExecutionRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		spec := runner.RunSpec{
			Command: req.Command,
			Args:    req.Args,
			Env:     req.Env,
			Cwd:     req.Cwd,
		}
		if spec.Command == "" {
			spec = demoSpec()
		}

		exec, err := deps.Executions.StartOneshot(context.Background(), spec)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, exec)
	}
}

func demoSpec() runner.RunSpec {
	return runner.RunSpec{
		Command: "bash",
		Args: []string{
			"-lc",
			`printf '\033[36mvibe-tree demo execution\033[0m\n'; for i in {1..200}; do printf '\033[32m[%03d]\033[0m hello\n' "$i"; sleep 0.05; done`,
		},
	}
}

// executionLogTailHandler 功能：读取某个 execution 日志尾部（用于断线补齐/切换回放）。
// 参数/返回：依赖于磁盘日志文件；返回 gin.HandlerFunc。
// 失败场景：文件不存在返回 404；读取失败返回 500。
// 副作用：读取磁盘文件。
func executionLogTailHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		executionID := c.Param("id")
		tail := parseTailBytes(c.Query("tail"))

		b, err := execution.ReadLogTail(executionID, tail)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "log not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Data(http.StatusOK, "text/plain; charset=utf-8", b)
	}
}

// cancelExecutionHandler 功能：请求取消指定 execution。
// 参数/返回：依赖 executions manager；返回 gin.HandlerFunc。
// 失败场景：execution 不存在返回 404；取消失败返回 500。
// 副作用：向子进程发送信号；最终状态通过 WS execution.exited 收敛。
func cancelExecutionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Executions == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "execution manager not configured"})
			return
		}

		executionID := c.Param("id")
		if err := deps.Executions.Cancel(executionID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// wsHandler 功能：升级 HTTP 连接为 WebSocket，并将连接注册进 Hub 以接收广播事件。
// 参数/返回：依赖 ws hub；返回 gin.HandlerFunc。
// 失败场景：升级失败时直接返回（客户端会收到握手错误）。
// 副作用：持有网络连接直至关闭；启动写协程与读循环。
func wsHandler(deps Deps) gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				return true
			}
			return allowDevOrigin(origin)
		},
	}

	return func(c *gin.Context) {
		if deps.Hub == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ws hub not configured"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		deps.Hub.Serve(conn)
	}
}

func allowDevOrigin(origin string) bool {
	origin = strings.ToLower(strings.TrimSpace(origin))
	return strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost:")
}

func parseTailBytes(raw string) int64 {
	const defaultTail int64 = 20000
	if raw == "" {
		return defaultTail
	}

	var v int64
	for _, r := range raw {
		if r < '0' || r > '9' {
			return defaultTail
		}
		v = v*10 + int64(r-'0')
		if v > 20*1024*1024 {
			return 20 * 1024 * 1024
		}
	}
	return v
}
