package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/config"
)

type translateTextRequest struct {
	Text string `json:"text" binding:"required"`
}

type translateTextResponse struct {
	Translated string `json:"translated"`
}

// translateTextHandler 功能：使用已配置的思考翻译模型将任意文本翻译为简体中文。
// 参数/返回：依赖 chat.Manager；返回 gin.HandlerFunc。
// 失败场景：翻译模型未配置返回 400；翻译调用失败返回 500。
// 副作用：向 LLM API 发起网络请求。
func translateTextHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Chat == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "chat manager not configured"})
			return
		}

		var req translateTextRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		req.Text = strings.TrimSpace(req.Text)
		if req.Text == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
			return
		}

		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if cfg.Basic == nil || cfg.Basic.ThinkingTranslation == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "翻译模型未配置，请先在基础设置中配置思考翻译模型"})
			return
		}

		tt := cfg.Basic.ThinkingTranslation
		modelCfg, source, _, ok := config.FindLLMModelByID(cfg.LLM, tt.ModelID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "翻译模型不存在，请检查基础设置"})
			return
		}

		provider := strings.ToLower(strings.TrimSpace(source.Provider))
		env := map[string]string{}
		switch provider {
		case "openai":
			env["OPENAI_API_KEY"] = strings.TrimSpace(source.APIKey)
		case "anthropic":
			env["ANTHROPIC_API_KEY"] = strings.TrimSpace(source.APIKey)
			if strings.TrimSpace(source.BaseURL) != "" {
				env["ANTHROPIC_BASE_URL"] = strings.TrimSpace(source.BaseURL)
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "翻译模型不支持该 provider：" + source.Provider})
			return
		}

		spec := chat.ThinkingTranslationSpec{
			Provider:       provider,
			Model:          strings.TrimSpace(modelCfg.Model),
			BaseURL:        strings.TrimSpace(source.BaseURL),
			Env:            env,
			OpenAIAPIStyle: modelCfg.OpenAIAPIStyle,
		}

		translated, err := deps.Chat.TranslateText(c.Request.Context(), spec, req.Text)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, translateTextResponse{Translated: translated})
	}
}
