package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
)

type basicSettingsResponse struct {
	ThinkingTranslation *thinkingTranslationPublic `json:"thinking_translation,omitempty"`
}

type thinkingTranslationPublic struct {
	SourceID       string   `json:"source_id"`
	Model          string   `json:"model"`
	TargetModelIDs []string `json:"target_model_ids"`
}

type putBasicSettingsRequest struct {
	ThinkingTranslation *putThinkingTranslation `json:"thinking_translation,omitempty"`
}

type putThinkingTranslation struct {
	SourceID       string   `json:"source_id"`
	Model          string   `json:"model"`
	TargetModelIDs []string `json:"target_model_ids"`
}

func getBasicSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildBasicSettingsResponse(cfg.Basic))
	}
}

func putBasicSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req putBasicSettingsRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		cfg, cfgPath, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		next := &config.BasicSettings{}
		if req.ThinkingTranslation != nil {
			next.ThinkingTranslation = &config.ThinkingTranslationSettings{
				SourceID:       strings.TrimSpace(req.ThinkingTranslation.SourceID),
				Model:          strings.TrimSpace(req.ThinkingTranslation.Model),
				TargetModelIDs: append([]string(nil), req.ThinkingTranslation.TargetModelIDs...),
			}
		}
		config.NormalizeBasicSettings(&next)
		if err := config.ValidateBasicSettings(next, cfg.LLM); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg.Basic = next
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, buildBasicSettingsResponse(cfg.Basic))
	}
}

func buildBasicSettingsResponse(basic *config.BasicSettings) basicSettingsResponse {
	resp := basicSettingsResponse{}
	if basic == nil || basic.ThinkingTranslation == nil {
		return resp
	}
	resp.ThinkingTranslation = &thinkingTranslationPublic{
		SourceID:       strings.TrimSpace(basic.ThinkingTranslation.SourceID),
		Model:          strings.TrimSpace(basic.ThinkingTranslation.Model),
		TargetModelIDs: append([]string(nil), basic.ThinkingTranslation.TargetModelIDs...),
	}
	return resp
}
