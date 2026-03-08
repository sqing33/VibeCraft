package api

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/skillcatalog"
)

type skillSettingsResponse struct {
	Skills []skillPublic `json:"skills"`
}

type skillPublic struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
	Source      string `json:"source,omitempty"`
}

func getSkillSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		discovered := skillcatalog.Discover()
		items := make([]skillPublic, 0, len(discovered))
		for _, item := range discovered {
			items = append(items, skillPublic{
				ID:          strings.TrimSpace(item.ID),
				Description: strings.TrimSpace(item.Description),
				Path:        strings.TrimSpace(item.Path),
				Source:      skillSourceFromPath(item.Path),
			})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		c.JSON(http.StatusOK, skillSettingsResponse{Skills: items})
	}
}

func skillSourceFromPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	sep := string(filepath.Separator)
	if strings.Contains(path, sep+".codex"+sep+"skills") {
		return "codex"
	}
	if strings.Contains(path, sep+".cc-switch"+sep+"skills") {
		return "cc-switch"
	}
	if strings.Contains(path, sep+".claude"+sep+"skills") {
		return "claude"
	}
	return "filesystem"
}
