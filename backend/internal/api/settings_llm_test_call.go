package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/runner"

	openai_option "github.com/openai/openai-go/option"
	openai_responses "github.com/openai/openai-go/responses"
	openai_shared "github.com/openai/openai-go/shared"
	openai "github.com/openai/openai-go"

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
		model := strings.TrimSpace(req.Model)
		apiKey := strings.TrimSpace(req.APIKey)
		sourceID := strings.TrimSpace(req.SourceID)
		baseURL := strings.TrimSpace(req.BaseURL)

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

		if apiKey == "" || baseURL == "" {
			cfg, _, err := config.LoadPersisted()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			llm := cfg.LLM
			if llm == nil || (len(llm.Sources) == 0 && len(llm.Models) == 0) {
				derived := deriveLLMFromExperts(cfg.Experts)
				llm = &derived
			}

			for _, s := range llm.Sources {
				if strings.TrimSpace(s.ID) != sourceID {
					continue
				}
				srcProvider := strings.ToLower(strings.TrimSpace(s.Provider))
				if srcProvider != "" && srcProvider != provider {
					c.JSON(http.StatusBadRequest, gin.H{"error": "source provider mismatch"})
					return
				}
				if apiKey == "" {
					apiKey = strings.TrimSpace(s.APIKey)
				}
				if baseURL == "" {
					baseURL = strings.TrimSpace(s.BaseURL)
				}
				break
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
			return
		}

		prompt := strings.TrimSpace(req.Prompt)
		if prompt == "" {
			prompt = "Reply with a single word: OK"
		}

		started := time.Now()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		var out string
		var err error
		switch provider {
		case "openai":
			baseURL = runner.NormalizeBaseURL("openai", baseURL)
			out, err = testOpenAI(ctx, model, baseURL, apiKey, prompt)
		case "anthropic":
			baseURL = runner.NormalizeBaseURL("anthropic", baseURL)
			out, err = testAnthropic(ctx, model, baseURL, apiKey, prompt)
		}
		if err != nil {
			// Avoid leaking secrets by returning only error string.
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

func testOpenAI(ctx context.Context, model, baseURL, apiKey, prompt string) (string, error) {
	client := openai.NewClient()
	opts := []openai_option.RequestOption{
		openai_option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, openai_option.WithBaseURL(baseURL))
	}

	body := openai_responses.ResponseNewParams{
		Model: openai_shared.ResponsesModel(model),
		Input: openai_responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		MaxOutputTokens: openai.Int(int64(32)),
	}

	stream := client.Responses.NewStreaming(ctx, body, opts...)
	if stream == nil {
		return "", errors.New("openai stream is nil")
	}
	defer stream.Close()

	var sb strings.Builder
	for stream.Next() {
		ev := stream.Current()
		switch ev.Type {
		case "response.output_text.delta":
			delta := ev.AsResponseOutputTextDelta().Delta
			if delta != "" {
				sb.WriteString(delta)
			}
		case "error":
			msg := strings.TrimSpace(ev.AsError().Message)
			if msg != "" {
				return "", errors.New(msg)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", err
	}
	return strings.TrimSpace(sb.String()), nil
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
	for _, b := range res.Content {
		if b.Type == "text" {
			sb.WriteString(b.AsText().Text)
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
