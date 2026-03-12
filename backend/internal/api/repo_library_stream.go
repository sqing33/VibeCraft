package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// repoLibraryStreamHandler 功能：提供 Repo Library analysis 状态变更的 SSE 事件流。
// 参数/返回：依赖 RepoLibraryStream broker；返回 gin.HandlerFunc。
// 失败场景：broker 缺失或 ResponseWriter 不支持 flush 时返回 500。
// 副作用：持有长连接，持续向客户端推送事件。
func repoLibraryStreamHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibraryStream == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library stream not configured"})
			return
		}

		w := c.Writer
		flusher, ok := w.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "stream not supported"})
			return
		}

		h := w.Header()
		h.Set("Content-Type", "text/event-stream; charset=utf-8")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		// Emit an initial comment so clients/proxies see bytes early and avoid buffering.
		_, _ = w.Write([]byte(": ok\n\n"))
		flusher.Flush()

		sub, ch := deps.RepoLibraryStream.Subscribe()
		defer deps.RepoLibraryStream.Unsubscribe(sub)

		ctx := c.Request.Context()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				_, _ = w.Write(msg)
				flusher.Flush()
			case <-ticker.C:
				_, _ = w.Write([]byte(": ping\n\n"))
				flusher.Flush()
			}
		}
	}
}
