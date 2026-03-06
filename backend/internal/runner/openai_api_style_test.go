package runner

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/openaicompat"
)

func TestStreamOpenAI_RejectsStructuredOutputOnChatCompletionsStyle(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai", APIKey: "sk-test"}},
		Models:  []config.LLMModelConfig{{ID: "model-a", Provider: "openai", Model: "gpt-4.1", SourceID: "openai-default", OpenAIAPIStyle: config.OpenAIAPIStyleChatCompletions, OpenAIAPIStyleDetectedAt: 123}},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	runner := NewSDKRunner()
	var out bytes.Buffer
	err = runner.streamOpenAI(context.Background(), SDKSpec{
		Provider:        "openai",
		Model:           "gpt-4.1",
		LLMModelID:      "model-a",
		Prompt:          "hello",
		OutputSchema:    "dag_v1",
		MaxOutputTokens: 32,
	}, map[string]string{"OPENAI_API_KEY": "sk-test"}, &out)
	if !errors.Is(err, openaicompat.ErrResponsesCompatibleEndpointRequired) {
		t.Fatalf("expected responses-only error, got %v", err)
	}
}
