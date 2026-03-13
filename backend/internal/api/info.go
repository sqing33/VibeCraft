package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/paths"
	"vibecraft/backend/internal/version"
)

type infoResponse struct {
	Version version.Info `json:"version"`
	Paths   struct {
		ConfigPath  string `json:"config_path"`
		DataDir     string `json:"data_dir"`
		LogsDir     string `json:"logs_dir"`
		StateDBPath string `json:"state_db_path"`
	} `json:"paths"`
	NowMs int64 `json:"now_ms"`
}

// infoHandler 功能：返回 daemon 运行信息（版本号 + XDG 路径），用于 UI 展示与排障。
// 参数/返回：依赖可为空（路径解析失败会返回 500）；成功返回 infoResponse。
// 失败场景：路径解析失败时返回 500。
// 副作用：读取环境变量与 home 目录信息（间接）。
func infoHandler(_ Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfgPath, err := config.Path()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		dataDir, err := paths.DataDir()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logsDir, err := paths.LogsDir()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		stateDBPath, err := paths.StateDBPath()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var res infoResponse
		res.Version = version.Current()
		res.Paths.ConfigPath = cfgPath
		res.Paths.DataDir = dataDir
		res.Paths.LogsDir = logsDir
		res.Paths.StateDBPath = stateDBPath
		res.NowMs = time.Now().UnixMilli()

		c.JSON(http.StatusOK, res)
	}
}
