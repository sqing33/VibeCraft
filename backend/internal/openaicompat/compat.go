package openaicompat

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	openai "github.com/openai/openai-go"
	openai_option "github.com/openai/openai-go/option"
	openai_responses "github.com/openai/openai-go/responses"
	openai_shared "github.com/openai/openai-go/shared"

	"vibe-tree/backend/internal/config"
)

type APIStyle string

const (
	APIStyleResponses       APIStyle = APIStyle(config.OpenAIAPIStyleResponses)
	APIStyleChatCompletions APIStyle = APIStyle(config.OpenAIAPIStyleChatCompletions)

	DetectionPrompt = "Reply with a single word: OK"
)

var (
	ErrResponsesCompatibleEndpointRequired = errors.New("responses-compatible endpoint is required")
	modelLocks                             sync.Map
	configWriteMu                          sync.Mutex
)

type TextRequest struct {
	Model           string
	BaseURL         string
	APIKey          string
	OrganizationID  string
	ProjectID       string
	Prompt          string
	Instructions    string
	MaxOutputTokens int
	Temperature     *float64
}

type Usage struct {
	TokenIn           *int64
	TokenOut          *int64
	CachedInputTokens *int64
}

func (s APIStyle) Valid() bool {
	return s == APIStyleResponses || s == APIStyleChatCompletions
}

func NormalizeAPIStyle(raw string) APIStyle {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(APIStyleResponses):
		return APIStyleResponses
	case string(APIStyleChatCompletions):
		return APIStyleChatCompletions
	default:
		return ""
	}
}

func IsEndpointMismatch(err error) bool {
	if err == nil {
		return false
	}
	var apierr *openai.Error
	if errors.As(err, &apierr) {
		if apierr.StatusCode == 404 || apierr.StatusCode == 405 || apierr.StatusCode == 501 {
			return true
		}
		payload := strings.ToLower(strings.TrimSpace(apierr.RawJSON()))
		message := strings.ToLower(strings.TrimSpace(apierr.Message))
		path := ""
		if apierr.Request != nil && apierr.Request.URL != nil {
			path = strings.ToLower(strings.TrimSpace(apierr.Request.URL.Path))
		}
		if strings.Contains(payload, "unsupported endpoint") || strings.Contains(payload, "unknown path") {
			return true
		}
		if strings.Contains(message, "unsupported endpoint") || strings.Contains(message, "unknown path") {
			return true
		}
		if strings.Contains(payload, "bad_response_status_code") && (strings.Contains(path, "/responses") || strings.Contains(path, "/chat/completions")) {
			return true
		}
		if strings.Contains(message, "not found") && (strings.Contains(path, "/responses") || strings.Contains(path, "/chat/completions")) {
			return true
		}
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "bad_response_status_code") && (strings.Contains(msg, "/v1/responses") || strings.Contains(msg, "/v1/chat/completions"))
}

func ProbeTextAPIStyle(ctx context.Context, req TextRequest) (APIStyle, string, error) {
	return probeOrdered(ctx, req, []APIStyle{APIStyleResponses, APIStyleChatCompletions})
}

func EnsureSavedModelAPIStyle(ctx context.Context, modelID string, detectReq TextRequest) (APIStyle, bool, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return APIStyleResponses, false, nil
	}
	mu := lockForModel(modelID)
	mu.Lock()
	defer mu.Unlock()

	style, err := loadSavedStyle(modelID)
	if err != nil {
		return "", false, err
	}
	if style.Valid() {
		return style, false, nil
	}
	detected, _, err := ProbeTextAPIStyle(ctx, detectReq)
	if err != nil {
		return "", false, err
	}
	if err := persistModelStyle(modelID, detected); err != nil {
		return "", false, err
	}
	return detected, true, nil
}

func ReprobeSavedModelAPIStyle(ctx context.Context, modelID string, current APIStyle, detectReq TextRequest) (APIStyle, bool, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return "", false, errors.New("model id is required")
	}
	mu := lockForModel(modelID)
	mu.Lock()
	defer mu.Unlock()

	styles := []APIStyle{APIStyleResponses, APIStyleChatCompletions}
	if current == APIStyleResponses {
		styles = []APIStyle{APIStyleChatCompletions}
	} else if current == APIStyleChatCompletions {
		styles = []APIStyle{APIStyleResponses}
	}
	detected, _, err := probeOrdered(ctx, detectReq, styles)
	if err != nil {
		return "", false, err
	}
	if err := persistModelStyle(modelID, detected); err != nil {
		return "", false, err
	}
	return detected, true, nil
}

func StreamText(ctx context.Context, style APIStyle, req TextRequest, onDelta func(string)) (Usage, error) {
	style = NormalizeAPIStyle(string(style))
	if !style.Valid() {
		return Usage{}, fmt.Errorf("unsupported api style %q", style)
	}
	req.Model = strings.TrimSpace(req.Model)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.BaseURL = normalizeOpenAIBaseURL(req.BaseURL)
	if req.Model == "" {
		return Usage{}, errors.New("openai model is required")
	}
	if req.Prompt == "" {
		return Usage{}, errors.New("prompt is required")
	}
	if onDelta == nil {
		onDelta = func(string) {}
	}
	switch style {
	case APIStyleResponses:
		return streamResponses(ctx, req, onDelta)
	case APIStyleChatCompletions:
		return streamChatCompletions(ctx, req, onDelta)
	default:
		return Usage{}, fmt.Errorf("unsupported api style %q", style)
	}
}

func CompleteText(ctx context.Context, style APIStyle, req TextRequest) (string, Usage, error) {
	var sb strings.Builder
	usage, err := StreamText(ctx, style, req, func(delta string) {
		if delta != "" {
			sb.WriteString(delta)
		}
	})
	return strings.TrimSpace(sb.String()), usage, err
}

func probeOrdered(ctx context.Context, req TextRequest, order []APIStyle) (APIStyle, string, error) {
	var lastErr error
	for _, style := range order {
		out, _, err := CompleteText(ctx, style, req)
		if err == nil {
			return style, out, nil
		}
		if !IsEndpointMismatch(err) {
			return "", "", err
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no supported OpenAI API style detected")
	}
	return "", "", lastErr
}

func streamResponses(ctx context.Context, req TextRequest, onDelta func(string)) (Usage, error) {
	client := openai.NewClient()
	opts := requestOptionsFromRequest(req)
	body := openai_responses.ResponseNewParams{
		Model: openai_shared.ResponsesModel(req.Model),
		Input: openai_responses.ResponseNewParamsInputUnion{OfString: openai.String(req.Prompt)},
	}
	if strings.TrimSpace(req.Instructions) != "" {
		body.Instructions = openai.String(strings.TrimSpace(req.Instructions))
	}
	if req.MaxOutputTokens > 0 {
		body.MaxOutputTokens = openai.Int(int64(req.MaxOutputTokens))
	}
	if req.Temperature != nil {
		body.Temperature = openai.Float(*req.Temperature)
	}
	stream := client.Responses.NewStreaming(ctx, body, opts...)
	if stream == nil {
		return Usage{}, errors.New("openai responses stream is nil")
	}
	defer stream.Close()
	var usage Usage
	for stream.Next() {
		ev := stream.Current()
		switch ev.Type {
		case "response.output_text.delta":
			delta := ev.AsResponseOutputTextDelta().Delta
			if delta != "" {
				onDelta(delta)
			}
		case "response.completed":
			u := ev.AsResponseCompleted().Response.Usage
			usage.TokenIn = int64Ptr(u.InputTokens)
			usage.TokenOut = int64Ptr(u.OutputTokens)
			usage.CachedInputTokens = int64Ptr(u.InputTokensDetails.CachedTokens)
		case "error":
			msg := strings.TrimSpace(ev.AsError().Message)
			if msg != "" {
				return Usage{}, errors.New(msg)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Usage{}, err
	}
	return usage, nil
}

func streamChatCompletions(ctx context.Context, req TextRequest, onDelta func(string)) (Usage, error) {
	client := openai.NewClient()
	opts := requestOptionsFromRequest(req)
	body := openai.ChatCompletionNewParams{
		Model:    req.Model,
		Messages: buildMessages(req.Instructions, req.Prompt),
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}
	if req.MaxOutputTokens > 0 {
		body.MaxCompletionTokens = openai.Int(int64(req.MaxOutputTokens))
	}
	if req.Temperature != nil {
		body.Temperature = openai.Float(*req.Temperature)
	}
	stream := client.Chat.Completions.NewStreaming(ctx, body, opts...)
	if stream == nil {
		return Usage{}, errors.New("openai chat completions stream is nil")
	}
	defer stream.Close()
	var usage Usage
	for stream.Next() {
		chunk := stream.Current()
		for _, choice := range chunk.Choices {
			delta := choice.Delta.Content
			if delta != "" {
				onDelta(delta)
			}
		}
		if chunk.JSON.Usage.Valid() {
			usage.TokenIn = int64Ptr(chunk.Usage.PromptTokens)
			usage.TokenOut = int64Ptr(chunk.Usage.CompletionTokens)
			usage.CachedInputTokens = int64Ptr(chunk.Usage.PromptTokensDetails.CachedTokens)
		}
	}
	if err := stream.Err(); err != nil {
		return Usage{}, err
	}
	return usage, nil
}

func buildMessages(system, prompt string) []openai.ChatCompletionMessageParamUnion {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, 2)
	if strings.TrimSpace(system) != "" {
		messages = append(messages, openai.SystemMessage(strings.TrimSpace(system)))
	}
	messages = append(messages, openai.UserMessage(strings.TrimSpace(prompt)))
	return messages
}

func requestOptionsFromRequest(req TextRequest) []openai_option.RequestOption {
	opts := make([]openai_option.RequestOption, 0, 4)
	if strings.TrimSpace(req.APIKey) != "" {
		opts = append(opts, openai_option.WithAPIKey(strings.TrimSpace(req.APIKey)))
	}
	if strings.TrimSpace(req.OrganizationID) != "" {
		opts = append(opts, openai_option.WithOrganization(strings.TrimSpace(req.OrganizationID)))
	}
	if strings.TrimSpace(req.ProjectID) != "" {
		opts = append(opts, openai_option.WithProject(strings.TrimSpace(req.ProjectID)))
	}
	if strings.TrimSpace(req.BaseURL) != "" {
		opts = append(opts, openai_option.WithBaseURL(strings.TrimSpace(req.BaseURL)))
	}
	return opts
}

func loadSavedStyle(modelID string) (APIStyle, error) {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return "", err
	}
	model, _, _, ok := config.FindLLMModelByID(cfg.LLM, modelID)
	if !ok {
		return "", fmt.Errorf("llm model %q not found", modelID)
	}
	return NormalizeAPIStyle(model.OpenAIAPIStyle), nil
}

func persistModelStyle(modelID string, style APIStyle) error {
	if !style.Valid() {
		return fmt.Errorf("unsupported api style %q", style)
	}
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, cfgPath, err := config.LoadPersisted()
	if err != nil {
		return err
	}
	_, _, idx, ok := config.FindLLMModelByID(cfg.LLM, modelID)
	if !ok {
		return fmt.Errorf("llm model %q not found", modelID)
	}
	cfg.LLM.Models[idx].OpenAIAPIStyle = string(style)
	cfg.LLM.Models[idx].OpenAIAPIStyleDetectedAt = time.Now().UnixMilli()
	return config.SaveTo(cfgPath, cfg)
}

func lockForModel(modelID string) *sync.Mutex {
	actual, _ := modelLocks.LoadOrStore(modelID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func int64Ptr(v int64) *int64 {
	return &v
}

func normalizeOpenAIBaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return s
	}
	u.Fragment = ""
	u.RawQuery = ""
	path := strings.TrimSuffix(strings.TrimSpace(u.Path), "/")
	if path == "" {
		path = "/v1"
	} else if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}
	u.Path = path
	return strings.TrimSuffix(u.String(), "/")
}
