package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// listExpertsHandler 功能：返回当前已配置 experts 列表（安全字段），用于 UI 下拉选择。
// 参数/返回：依赖 deps.Experts；成功返回 []expert.PublicExpert。
// 失败场景：expert registry 未配置时返回 500。
// 副作用：无。
func listExpertsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}
		c.JSON(http.StatusOK, deps.Experts.ListPublic())
	}
}
