package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/openai/openai-go"
	openai_option "github.com/openai/openai-go/option"
	openai_responses "github.com/openai/openai-go/responses"
	openai_shared "github.com/openai/openai-go/shared"

	"vibe-tree/backend/internal/dag"
	"vibe-tree/backend/internal/expertschema"
	"vibe-tree/backend/internal/openaicompat"
)

// SDKRunner 功能：使用 OpenAI/Anthropic 官方 SDK 执行一次性生成任务，并以流式方式输出到 execution 日志。
// 参数/返回：根据 spec.SDK.Provider 选择 provider；返回可取消/可等待的句柄。
// 失败场景：provider/model 配置缺失、鉴权失败、网络错误或输出 schema 不支持时返回 error。
// 副作用：发起外部网络请求并产生输出流。
type SDKRunner struct {
	openaiClient    openai.Client
	anthropicClient anthropic.Client
}

// NewSDKRunner 功能：创建默认 SDKRunner（client options 从环境变量读取，单次执行可被 spec.Env 覆盖）。
// 参数/返回：无入参；返回 SDKRunner。
// 失败场景：无（仅构造 client）。
// 副作用：读取环境变量（由 SDK 初始化逻辑决定）。
func NewSDKRunner() SDKRunner {
	return SDKRunner{
		openaiClient:    openai.NewClient(),
		anthropicClient: anthropic.NewClient(),
	}
}

func (r SDKRunner) StartOneshot(ctx context.Context, spec RunSpec) (ProcessHandle, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if spec.SDK == nil {
		return nil, errors.New("sdk spec is required")
	}

	provider := strings.ToLower(strings.TrimSpace(spec.SDK.Provider))
	if provider == "" {
		return nil, errors.New("sdk provider is required")
	}
	if strings.TrimSpace(spec.SDK.Model) == "" && provider != "demo" {
		return nil, errors.New("sdk model is required")
	}

	startedAt := time.Now()
	runCtx, cancel := context.WithCancel(ctx)

	pr, pw := io.Pipe()
	h := &sdkProcessHandle{
		output:    pr,
		writer:    pw,
		cancel:    cancel,
		startedAt: startedAt,
		waitCh:    make(chan struct{}),
	}

	go h.run(func() error {
		attempts := make([]SDKFallback, 0, 1+len(spec.SDKFallbacks))
		attempts = append(attempts, SDKFallback{SDK: *spec.SDK, Env: cloneEnv(spec.Env)})
		attempts = append(attempts, spec.SDKFallbacks...)
		var lastErr error
		for idx, attempt := range attempts {
			attemptProvider := strings.ToLower(strings.TrimSpace(attempt.SDK.Provider))
			attemptModel := strings.TrimSpace(attempt.SDK.Model)
			if idx == 0 {
				_, _ = fmt.Fprintf(pw, "\x1b[36m[sdk:%s]\x1b[0m model=%s\n", attemptProvider, attemptModel)
			} else {
				_, _ = fmt.Fprintf(pw, "\n[sdk] fallback retry #%d -> %s/%s\n", idx, attemptProvider, attemptModel)
			}
			var err error
			switch attemptProvider {
			case "openai":
				err = r.streamOpenAI(runCtx, attempt.SDK, attempt.Env, pw)
			case "anthropic":
				err = r.streamAnthropic(runCtx, attempt.SDK, attempt.Env, pw)
			case "demo":
				err = r.streamDemo(runCtx, attempt.SDK, pw)
			default:
				err = fmt.Errorf("unsupported sdk provider %q", attemptProvider)
			}
			if err == nil {
				return nil
			}
			lastErr = err
		}
		return lastErr
	})

	return h, nil
}

type sdkProcessHandle struct {
	output *io.PipeReader
	writer *io.PipeWriter
	cancel context.CancelFunc

	startedAt time.Time

	closeOnce sync.Once

	mu     sync.Mutex
	waitCh chan struct{}
	exit   ExitResult
	err    error
}

func (h *sdkProcessHandle) PID() int { return 0 }

func (h *sdkProcessHandle) Output() io.ReadCloser { return h.output }

func (h *sdkProcessHandle) Wait() (ExitResult, error) {
	<-h.waitCh
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exit, h.err
}

func (h *sdkProcessHandle) Cancel(_ time.Duration) error {
	if h.cancel != nil {
		h.cancel()
	}
	h.closeOnce.Do(func() {
		_ = h.writer.CloseWithError(context.Canceled)
	})
	return nil
}

func (h *sdkProcessHandle) WriteInput(_ []byte) (int, error) {
	return 0, errors.New("sdk runner does not support stdin")
}

func (h *sdkProcessHandle) Close() error {
	if h.cancel != nil {
		h.cancel()
	}
	h.closeOnce.Do(func() {
		_ = h.writer.Close()
	})
	if h.output != nil {
		_ = h.output.Close()
	}
	return nil
}

func (h *sdkProcessHandle) run(fn func() error) {
	defer close(h.waitCh)
	defer func() {
		h.closeOnce.Do(func() {
			_ = h.writer.Close()
		})
	}()

	err := fn()
	endedAt := time.Now()

	// Cancel/timeout 由上层 ctx/CancelRequested 收敛为 execution 状态，这里避免把 ctx error 当作失败。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		err = nil
	}

	exitCode := 0
	if err != nil {
		exitCode = 1
		_, _ = fmt.Fprintf(h.writer, "\n[sdk] error: %v\n", err)
	}

	h.mu.Lock()
	h.exit = ExitResult{
		ExitCode:  exitCode,
		Signal:    "",
		StartedAt: h.startedAt,
		EndedAt:   endedAt,
	}
	h.err = err
	h.mu.Unlock()
}

func (r SDKRunner) streamOpenAI(ctx context.Context, sdk SDKSpec, env map[string]string, out io.Writer) error {
	model := strings.TrimSpace(sdk.Model)
	if model == "" {
		return errors.New("openai model is required")
	}
	if strings.TrimSpace(sdk.OutputSchema) == "" {
		style, err := r.ensureOpenAIAPIStyle(ctx, sdk, env)
		if err != nil {
			return err
		}
		return r.streamOpenAIPlainText(ctx, sdk, env, out, style)
	}
	style, err := r.ensureOpenAIAPIStyle(ctx, sdk, env)
	if err != nil {
		return err
	}
	if style == openaicompat.APIStyleChatCompletions {
		return openaicompat.ErrResponsesCompatibleEndpointRequired
	}
	if err := r.streamOpenAIResponses(ctx, sdk, env, out); err != nil {
		if openaicompat.IsEndpointMismatch(err) && strings.TrimSpace(sdk.LLMModelID) != "" {
			detectReq := openAITextRequest(sdk, env, openaicompat.DetectionPrompt)
			style, _, retryErr := openaicompat.ReprobeSavedModelAPIStyle(ctx, sdk.LLMModelID, openaicompat.APIStyleResponses, detectReq)
			if retryErr != nil {
				return retryErr
			}
			if style == openaicompat.APIStyleChatCompletions {
				return openaicompat.ErrResponsesCompatibleEndpointRequired
			}
			return r.streamOpenAIResponses(ctx, sdk, env, out)
		}
		return err
	}
	return nil
}

func (r SDKRunner) streamOpenAIPlainText(ctx context.Context, sdk SDKSpec, env map[string]string, out io.Writer, style openaicompat.APIStyle) error {
	request := openAITextRequest(sdk, env, sdk.Prompt)
	_, err := openaicompat.StreamText(ctx, style, request, func(delta string) {
		if delta == "" {
			return
		}
		_, _ = io.WriteString(out, delta)
	})
	if err != nil && openaicompat.IsEndpointMismatch(err) && strings.TrimSpace(sdk.LLMModelID) != "" {
		detectReq := openAITextRequest(sdk, env, openaicompat.DetectionPrompt)
		nextStyle, _, retryErr := openaicompat.ReprobeSavedModelAPIStyle(ctx, sdk.LLMModelID, style, detectReq)
		if retryErr != nil {
			return retryErr
		}
		_, err = openaicompat.StreamText(ctx, nextStyle, request, func(delta string) {
			if delta == "" {
				return
			}
			_, _ = io.WriteString(out, delta)
		})
	}
	return err
}

func (r SDKRunner) streamOpenAIResponses(ctx context.Context, sdk SDKSpec, env map[string]string, out io.Writer) error {
	model := strings.TrimSpace(sdk.Model)
	if model == "" {
		return errors.New("openai model is required")
	}

	body := openai_responses.ResponseNewParams{
		Model: openai_shared.ResponsesModel(model),
		Input: openai_responses.ResponseNewParamsInputUnion{
			OfString: openai.String(sdk.Prompt),
		},
	}
	if strings.TrimSpace(sdk.Instructions) != "" {
		body.Instructions = openai.String(sdk.Instructions)
	}
	if sdk.MaxOutputTokens > 0 {
		body.MaxOutputTokens = openai.Int(int64(sdk.MaxOutputTokens))
	}
	if sdk.Temperature != nil {
		body.Temperature = openai.Float(*sdk.Temperature)
	}

	if strings.TrimSpace(sdk.OutputSchema) != "" {
		schema, err := builtinOutputSchema(sdk.OutputSchema)
		if err != nil {
			return err
		}
		body.Text = openai_responses.ResponseTextConfigParam{
			Format: openai_responses.ResponseFormatTextConfigParamOfJSONSchema(sdk.OutputSchema, schema),
		}
	}

	opts := make([]openai_option.RequestOption, 0, 4)
	if v := strings.TrimSpace(env["OPENAI_API_KEY"]); v != "" {
		opts = append(opts, openai_option.WithAPIKey(v))
	}
	if v := strings.TrimSpace(env["OPENAI_ORG_ID"]); v != "" {
		opts = append(opts, openai_option.WithOrganization(v))
	}
	if v := strings.TrimSpace(env["OPENAI_PROJECT_ID"]); v != "" {
		opts = append(opts, openai_option.WithProject(v))
	}
	if baseURL := strings.TrimSpace(sdk.BaseURL); baseURL != "" {
		opts = append(opts, openai_option.WithBaseURL(NormalizeBaseURL("openai", baseURL)))
	} else if baseURL := strings.TrimSpace(env["OPENAI_BASE_URL"]); baseURL != "" {
		opts = append(opts, openai_option.WithBaseURL(NormalizeBaseURL("openai", baseURL)))
	}

	stream := r.openaiClient.Responses.NewStreaming(ctx, body, opts...)
	if stream == nil {
		return errors.New("openai stream is nil")
	}
	defer stream.Close()

	for stream.Next() {
		ev := stream.Current()
		switch ev.Type {
		case "response.output_text.delta":
			delta := ev.AsResponseOutputTextDelta().Delta
			if delta == "" {
				continue
			}
			if _, err := io.WriteString(out, delta); err != nil {
				return err
			}
		case "error":
			msg := strings.TrimSpace(ev.AsError().Message)
			if msg != "" {
				_, _ = fmt.Fprintf(out, "\n[openai:error] %s\n", msg)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return err
	}
	return nil
}

func (r SDKRunner) ensureOpenAIAPIStyle(ctx context.Context, sdk SDKSpec, env map[string]string) (openaicompat.APIStyle, error) {
	if strings.TrimSpace(sdk.LLMModelID) == "" {
		return openaicompat.APIStyleResponses, nil
	}
	detectReq := openAITextRequest(sdk, env, openaicompat.DetectionPrompt)
	style, _, err := openaicompat.EnsureSavedModelAPIStyle(ctx, sdk.LLMModelID, detectReq)
	return style, err
}

func openAITextRequest(sdk SDKSpec, env map[string]string, prompt string) openaicompat.TextRequest {
	baseURL := strings.TrimSpace(sdk.BaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(env["OPENAI_BASE_URL"])
	}
	return openaicompat.TextRequest{
		Model:           strings.TrimSpace(sdk.Model),
		BaseURL:         baseURL,
		APIKey:          strings.TrimSpace(env["OPENAI_API_KEY"]),
		OrganizationID:  strings.TrimSpace(env["OPENAI_ORG_ID"]),
		ProjectID:       strings.TrimSpace(env["OPENAI_PROJECT_ID"]),
		Prompt:          strings.TrimSpace(prompt),
		Instructions:    strings.TrimSpace(sdk.Instructions),
		MaxOutputTokens: sdk.MaxOutputTokens,
		Temperature:     sdk.Temperature,
	}
}

func (r SDKRunner) streamAnthropic(ctx context.Context, sdk SDKSpec, env map[string]string, out io.Writer) error {
	model := strings.TrimSpace(sdk.Model)
	if model == "" {
		return errors.New("anthropic model is required")
	}

	maxTokens := sdk.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	body := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(sdk.Prompt)),
		},
	}
	if strings.TrimSpace(sdk.Instructions) != "" {
		body.System = []anthropic.TextBlockParam{{Text: sdk.Instructions}}
	}
	if sdk.Temperature != nil {
		body.Temperature = anthropic.Float(*sdk.Temperature)
	}
	if strings.TrimSpace(sdk.OutputSchema) != "" {
		schema, err := builtinOutputSchema(sdk.OutputSchema)
		if err != nil {
			return err
		}
		body.OutputConfig = anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		}
	}

	opts := make([]anthropic_option.RequestOption, 0, 2)
	if v := strings.TrimSpace(env["ANTHROPIC_API_KEY"]); v != "" {
		opts = append(opts, anthropic_option.WithAPIKey(v))
	}
	if baseURL := strings.TrimSpace(sdk.BaseURL); baseURL != "" {
		opts = append(opts, anthropic_option.WithBaseURL(NormalizeBaseURL("anthropic", baseURL)))
	} else if baseURL := strings.TrimSpace(env["ANTHROPIC_BASE_URL"]); baseURL != "" {
		opts = append(opts, anthropic_option.WithBaseURL(NormalizeBaseURL("anthropic", baseURL)))
	}

	stream := r.anthropicClient.Messages.NewStreaming(ctx, body, opts...)
	if stream == nil {
		return errors.New("anthropic stream is nil")
	}
	defer stream.Close()

	for stream.Next() {
		ev := stream.Current()
		if ev.Type != "content_block_delta" {
			continue
		}

		de := ev.AsContentBlockDelta()
		switch strings.TrimSpace(de.Delta.Type) {
		case "text_delta":
			txt := de.Delta.AsTextDelta().Text
			if txt == "" {
				continue
			}
			if _, err := io.WriteString(out, txt); err != nil {
				return err
			}
		case "input_json_delta":
			partial := de.Delta.AsInputJSONDelta().PartialJSON
			if partial == "" {
				continue
			}
			if _, err := io.WriteString(out, partial); err != nil {
				return err
			}
		}
	}
	if err := stream.Err(); err != nil {
		return err
	}
	return nil
}

func (r SDKRunner) streamDemo(ctx context.Context, sdk SDKSpec, out io.Writer) error {
	_ = ctx // demo does not need ctx right now; keep signature consistent for future cancellation points.
	if strings.TrimSpace(sdk.OutputSchema) != "" {
		// 输出一个稳定 DAG，便于本地链路验证（不依赖网络/密钥）。
		_, err := io.WriteString(out, `{
  "workflow_title": "",
  "nodes": [
    {
      "id": "n1",
      "title": "Step 1",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "demo",
      "prompt": "echo '[n1] hello'; sleep 0.02; echo '[n1] done'"
    },
    {
      "id": "n2",
      "title": "Step 2",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "demo",
      "prompt": "echo '[n2] hello'; sleep 0.02; echo '[n2] done'"
    }
  ],
  "edges": [
    { "from": "n1", "to": "n2", "type": "success", "source_handle": null, "target_handle": null }
  ]
}
`)
		return err
	}

	prompt := strings.TrimSpace(sdk.Prompt)
	if prompt == "" {
		prompt = "demo task"
	}
	_, err := fmt.Fprintf(out, "[demo]\n目标：%s\n\n摘要：已完成一次本地 demo 执行。\n建议：检查日志与产物摘要后决定是否继续下一轮。\n", prompt)
	return err
}

func builtinOutputSchema(name string) (map[string]any, error) {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "dag_v1":
		return dag.JSONSchemaV1(), nil
	case "expert_builder_v1":
		return expertschema.ExpertBuilderJSONSchemaV1(), nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported output_schema %q", name)
	}
}

func cloneEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}
