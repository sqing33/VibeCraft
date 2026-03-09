package api

import (
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
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
	Enabled     bool   `json:"enabled"`
}

type putSkillSettingsRequest struct {
	Skills []skillPublic `json:"skills"`
}

func getSkillSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildSkillSettingsResponse(cfg, skillcatalog.Discover()))
	}
}

func putSkillSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req putSkillSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, cfgPath, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		discovered := skillcatalog.Discover()
		discoveredByID := make(map[string]skillcatalog.Entry, len(discovered))
		for _, item := range discovered {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				continue
			}
			discoveredByID[id] = item
		}
		next := make([]config.SkillBindingConfig, 0, len(req.Skills))
		for index, item := range req.Skills {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "skills[" + strconv.Itoa(index) + "].id is required"})
				return
			}
			entry := discoveredByID[id]
			next = append(next, config.SkillBindingConfig{
				ID:          id,
				Description: firstNonEmptySkillField(strings.TrimSpace(item.Description), strings.TrimSpace(entry.Description)),
				Path:        firstNonEmptySkillField(strings.TrimSpace(item.Path), strings.TrimSpace(entry.Path)),
				Source:      firstNonEmptySkillField(strings.TrimSpace(item.Source), skillSourceFromPath(entry.Path)),
				Enabled:     item.Enabled,
			})
		}
		cfg.SkillBindings = next
		if err := config.NormalizeSkillBindings(&cfg.SkillBindings, cfg.CLITools); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildSkillSettingsResponse(cfg, discovered))
	}
}

func installSkillSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<20)
		if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}

		var installErr error
		if c.Request.MultipartForm != nil && len(c.Request.MultipartForm.File["archive"]) > 0 {
			_, installErr = skillcatalog.InstallFromZip(c.Request.MultipartForm.File["archive"][0])
		} else if c.Request.MultipartForm != nil && len(c.Request.MultipartForm.File["files"]) > 0 {
			_, installErr = skillcatalog.InstallFromUploadedFiles(c.Request.MultipartForm.File["files"], c.Request.MultipartForm.Value["paths"])
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "archive or files is required"})
			return
		}
		if installErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": installErr.Error()})
			return
		}

		cfg, cfgPath, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		discovered := skillcatalog.Discover()
		cfg.SkillBindings = mergeSkillBindingsWithDiscovered(cfg.SkillBindings, discovered)
		if err := config.NormalizeSkillBindings(&cfg.SkillBindings, cfg.CLITools); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildSkillSettingsResponse(cfg, discovered))
	}
}

func buildSkillSettingsResponse(cfg config.Config, discovered []skillcatalog.Entry) skillSettingsResponse {
	if len(discovered) == 0 {
		discovered = skillcatalog.Discover()
	}
	bindings := make(map[string]config.SkillBindingConfig, len(cfg.SkillBindings))
	for _, item := range cfg.SkillBindings {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		bindings[id] = item
	}
	items := make([]skillPublic, 0, len(discovered))
	for _, item := range discovered {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		binding, hasBinding := bindings[id]
		items = append(items, skillPublic{
			ID:          id,
			Description: firstNonEmptySkillField(strings.TrimSpace(binding.Description), strings.TrimSpace(item.Description)),
			Path:        firstNonEmptySkillField(strings.TrimSpace(item.Path), strings.TrimSpace(binding.Path)),
			Source:      firstNonEmptySkillField(skillSourceFromPath(item.Path), strings.TrimSpace(binding.Source)),
			Enabled:     !hasBinding || binding.Enabled,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return skillSettingsResponse{Skills: items}
}

func mergeSkillBindingsWithDiscovered(existing []config.SkillBindingConfig, discovered []skillcatalog.Entry) []config.SkillBindingConfig {
	bindings := make(map[string]config.SkillBindingConfig, len(existing))
	for _, item := range existing {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		bindings[id] = item
	}
	for _, item := range discovered {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		binding, ok := bindings[id]
		if !ok {
			binding = config.SkillBindingConfig{ID: id, Enabled: true}
		}
		binding.Description = strings.TrimSpace(item.Description)
		binding.Path = strings.TrimSpace(item.Path)
		binding.Source = skillSourceFromPath(item.Path)
		bindings[id] = binding
	}
	out := make([]config.SkillBindingConfig, 0, len(bindings))
	for _, item := range bindings {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func firstNonEmptySkillField(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
