package server

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/api"
	"vibe-tree/backend/internal/logx"
)

type Options struct {
	DevCORS bool
}

// New 功能：构建 Gin Engine，并挂载中间件（恢复、请求日志、dev CORS）与 API 路由。
// 参数/返回：opts 控制 dev 行为；deps 注入 API 依赖；返回 *gin.Engine。
// 失败场景：无（路由注册阶段不返回错误）。
// 副作用：初始化路由与中间件链。
func New(opts Options, deps api.Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	if opts.DevCORS {
		r.Use(cors.New(cors.Config{
			AllowOriginFunc:  allowDevOrigin,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	v1 := r.Group("/api/v1")
	api.Register(v1, deps)

	return r
}

// DevCORSFromEnv 功能：根据环境变量判断是否启用开发期 CORS。
// 参数/返回：读取 `VIBE_TREE_ENV`；dev/development/空值返回 true。
// 失败场景：无。
// 副作用：读取环境变量。
func DevCORSFromEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("VIBE_TREE_ENV")))
	return env == "" || env == "dev" || env == "development"
}

func allowDevOrigin(origin string) bool {
	origin = strings.ToLower(strings.TrimSpace(origin))
	return strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost:")
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		latency := time.Since(startedAt).Truncate(time.Millisecond)
		logx.Info(
			"http",
			"request",
			"HTTP 请求",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
		)
	}
}
