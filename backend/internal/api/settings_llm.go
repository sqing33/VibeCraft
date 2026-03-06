package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
)

type llmSettingsResponse struct {
	Sources []llmSourcePublic `json:"sources"`
	Models  []llmModelPublic  `json:"models"`
}

type llmSourcePublic struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url,omitempty"`
	HasKey    bool   `json:"has_key"`
	MaskedKey string `json:"masked_key,omitempty"`
}

type llmModelPublic struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	SourceID string `json:"source_id"`
}

type putLLMSettingsRequest struct {
	Sources []putLLMSource `json:"sources"`
	Models  []putLLMModel  `json:"models"`
}

type putLLMSource struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Provider string  `json:"provider"`
	BaseURL  string  `json:"base_url,omitempty"`
	APIKey   *string `json:"api_key,omitempty"`
}

type putLLMModel struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	SourceID string `json:"source_id"`
}

func getLLMSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		llm := cfg.LLM
		if llm == nil {
			c.JSON(http.StatusOK, llmSettingsResponse{Sources: []llmSourcePublic{}, Models: []llmModelPublic{}})
			return
		}
		c.JSON(http.StatusOK, buildLLMSettingsResponse(*llm))
	}
}

func putLLMSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}

		var req putLLMSettingsRequest
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

		existingKeyByID := make(map[string]string)
		if cfg.LLM != nil {
			for _, s := range cfg.LLM.Sources {
				existingKeyByID[strings.TrimSpace(s.ID)] = s.APIKey
			}
		}

		next := &config.LLMSettings{
			Sources: make([]config.LLMSourceConfig, 0, len(req.Sources)),
			Models:  make([]config.LLMModelConfig, 0, len(req.Models)),
		}

		for _, s := range req.Sources {
			id := strings.TrimSpace(s.ID)
			apiKey := ""
			if s.APIKey == nil {
				apiKey = existingKeyByID[id]
			} else {
				apiKey = strings.TrimSpace(*s.APIKey)
			}
			next.Sources = append(next.Sources, config.LLMSourceConfig{
				ID:       id,
				Label:    strings.TrimSpace(s.Label),
				Provider: strings.TrimSpace(s.Provider),
				BaseURL:  strings.TrimSpace(s.BaseURL),
				APIKey:   apiKey,
			})
		}
		for _, m := range req.Models {
			next.Models = append(next.Models, config.LLMModelConfig{
				ID:       strings.TrimSpace(m.ID),
				Label:    strings.TrimSpace(m.Label),
				Provider: strings.TrimSpace(m.Provider),
				Model:    strings.TrimSpace(m.Model),
				SourceID: strings.TrimSpace(m.SourceID),
			})
		}

		if err := config.NormalizeLLMSettings(next); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		cfg.LLM = next
		config.ReconcileBasicSettingsWithLLM(&cfg.Basic, cfg.LLM)
		if err := config.MirrorLLMToExperts(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		deps.Experts.Reload(cfg)
		c.JSON(http.StatusOK, buildLLMSettingsResponse(*cfg.LLM))
	}
}

func buildLLMSettingsResponse(llm config.LLMSettings) llmSettingsResponse {
	res := llmSettingsResponse{
		Sources: make([]llmSourcePublic, 0, len(llm.Sources)),
		Models:  make([]llmModelPublic, 0, len(llm.Models)),
	}

	for _, s := range llm.Sources {
		key := strings.TrimSpace(s.APIKey)
		res.Sources = append(res.Sources, llmSourcePublic{
			ID:        strings.TrimSpace(s.ID),
			Label:     strings.TrimSpace(s.Label),
			Provider:  strings.TrimSpace(s.Provider),
			BaseURL:   strings.TrimSpace(s.BaseURL),
			HasKey:    key != "",
			MaskedKey: maskKey(key),
		})
	}
	for _, m := range llm.Models {
		res.Models = append(res.Models, llmModelPublic{
			ID:       strings.TrimSpace(m.ID),
			Label:    strings.TrimSpace(m.Label),
			Provider: strings.TrimSpace(m.Provider),
			Model:    strings.TrimSpace(m.Model),
			SourceID: strings.TrimSpace(m.SourceID),
		})
	}
	return res
}

func maskKey(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

func deriveLLMFromExperts(experts []config.ExpertConfig) config.LLMSettings {
	type sourceKey struct {
		provider string
		baseURL  string
		apiKey   string
	}

	sourceIDByKey := make(map[sourceKey]string)
	sources := make([]config.LLMSourceConfig, 0)
	models := make([]config.LLMModelConfig, 0)

	nextID := func(provider string, n int) string {
		if n == 0 {
			return provider + "-default"
		}
		return provider + "-alt-" + strconv.Itoa(n+1)
	}

	countByProvider := make(map[string]int)

	for _, e := range experts {
		provider := strings.ToLower(strings.TrimSpace(e.Provider))
		if provider != "openai" && provider != "anthropic" {
			continue
		}

		baseURL := strings.TrimSpace(e.BaseURL)
		apiKey := ""
		if provider == "openai" {
			apiKey = strings.TrimSpace(e.Env["OPENAI_API_KEY"])
		} else {
			apiKey = strings.TrimSpace(e.Env["ANTHROPIC_API_KEY"])
		}
		if strings.Contains(apiKey, "${") {
			apiKey = ""
		}

		k := sourceKey{provider: provider, baseURL: baseURL, apiKey: apiKey}
		sourceID, ok := sourceIDByKey[k]
		if !ok {
			n := countByProvider[provider]
			countByProvider[provider] = n + 1
			sourceID = nextID(provider, n)
			sourceIDByKey[k] = sourceID
			sources = append(sources, config.LLMSourceConfig{
				ID:       sourceID,
				Label:    sourceID,
				Provider: provider,
				BaseURL:  baseURL,
				APIKey:   apiKey,
			})
		}

		id := strings.TrimSpace(e.ID)
		model := strings.TrimSpace(e.Model)
		if id == "" || model == "" {
			continue
		}
		label := strings.TrimSpace(e.Label)
		models = append(models, config.LLMModelConfig{
			ID:       id,
			Label:    label,
			Provider: provider,
			Model:    model,
			SourceID: sourceID,
		})
	}

	return config.LLMSettings{Sources: sources, Models: models}
}
