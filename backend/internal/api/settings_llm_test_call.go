package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/openaicompat"
	"vibe-tree/backend/internal/runner"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"
)

type llmTestRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url,omitempty"`
	SourceID string `json:"source_id,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

type llmTestResponse struct {
	OK        bool   `json:"ok"`
	Output    string `json:"output"`
	LatencyMs int64  `json:"latency_ms"`
}

func llmTestHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req llmTestRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		provider := strings.ToLower(strings.TrimSpace(req.Provider))
		model := strings.ToLower(strings.TrimSpace(req.Model))
		apiKey := strings.TrimSpace(req.APIKey)
		sourceID := strings.TrimSpace(req.SourceID)
		baseURL := strings.TrimSpace(req.BaseURL)
		var llm *config.LLMSettings
		var savedModelID string

		if provider != "openai" && provider != "anthropic" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider must be openai or anthropic"})
			return
		}
		if model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
			return
		}
		if apiKey == "" && sourceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "api_key or source_id is required"})
			return
		}
		if baseURL != "" {
			u, err := url.Parse(baseURL)
			if err != nil || u == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "base_url is invalid"})
				return
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "base_url must start with http:// or https://"})
				return
			}
		}

		cfg, _, err := config.LoadPersisted()
		if err == nil {
			llm = cfg.LLM
			if llm == nil || (len(llm.Sources) == 0 && len(llm.Models) == 0) {
				derived := deriveLLMFromExperts(cfg.Experts)
				llm = &derived
			}
		}

		if apiKey == "" || baseURL == "" {
			if llm == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			for _, source := range llm.Sources {
				if strings.TrimSpace(source.ID) != sourceID {
					continue
				}
				sourceProvider := strings.ToLower(strings.TrimSpace(source.Provider))
				if sourceProvider != "" && sourceProvider != provider {
					c.JSON(http.StatusBadRequest, gin.H{"error": "source provider mismatch"})
					return
				}
				if apiKey == "" {
					apiKey = strings.TrimSpace(source.APIKey)
				}
				if baseURL == "" {
					baseURL = strings.TrimSpace(source.BaseURL)
				}
				break
			}
		}

		if provider == "openai" && llm != nil {
			if modelCfg, _, _, ok := config.FindLLMModelByIdentity(llm, provider, sourceID, model); ok {
				savedModelID = strings.TrimSpace(modelCfg.ID)
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
			return
		}

		prompt := strings.TrimSpace(req.Prompt)
		if prompt == "" {
			prompt = openaicompat.DetectionPrompt
		}

		started := time.Now()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		var out string
		switch provider {
		case "openai":
			baseURL = runner.NormalizeBaseURL("openai", baseURL)
			out, err = testOpenAI(ctx, savedModelID, model, baseURL, apiKey, prompt)
		case "anthropic":
			baseURL = runner.NormalizeBaseURL("anthropic", baseURL)
			out, err = testAnthropic(ctx, model, baseURL, apiKey, prompt)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		out = truncate(out, 200)
		c.JSON(http.StatusOK, llmTestResponse{
			OK:        true,
			Output:    out,
			LatencyMs: time.Since(started).Milliseconds(),
		})
	}
}

func testOpenAI(ctx context.Context, savedModelID, model, baseURL, apiKey, prompt string) (string, error) {
	request := openaicompat.TextRequest{
		Model:           model,
		BaseURL:         baseURL,
		APIKey:          apiKey,
		Prompt:          prompt,
		MaxOutputTokens: 32,
	}
	if strings.TrimSpace(savedModelID) == "" {
		_, out, err := openaicompat.ProbeTextAPIStyle(ctx, request)
		return out, err
	}
	detectRequest := request
	detectRequest.Prompt = openaicompat.DetectionPrompt
	style, _, err := openaicompat.EnsureSavedModelAPIStyle(ctx, savedModelID, detectRequest)
	if err != nil {
		return "", err
	}
	out, _, err := openaicompat.CompleteText(ctx, style, request)
	if err != nil && openaicompat.IsEndpointMismatch(err) {
		style, _, retryErr := openaicompat.ReprobeSavedModelAPIStyle(ctx, savedModelID, style, detectRequest)
		if retryErr != nil {
			return "", retryErr
		}
		out, _, err = openaicompat.CompleteText(ctx, style, request)
	}
	return out, err
}

func testAnthropic(ctx context.Context, model, baseURL, apiKey, prompt string) (string, error) {
	client := anthropic.NewClient()
	opts := []anthropic_option.RequestOption{
		anthropic_option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, anthropic_option.WithBaseURL(baseURL))
	}

	body := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 32,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}

	res, err := client.Messages.New(ctx, body, opts...)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, block := range res.Content {
		if block.Type == "text" {
			sb.WriteString(block.AsText().Text)
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}
