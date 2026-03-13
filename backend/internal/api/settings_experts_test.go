package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"vibecraft/backend/internal/config"
)

func TestExpertSettings_SaveCustomExpert(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "openai-default", Provider: "openai", APIKey: "test-openai"},
			{ID: "anthropic-default", Provider: "anthropic", APIKey: "test-anthropic"},
		},
		Models: []config.LLMModelConfig{
			{ID: "ui-primary", Label: "UI Primary", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"},
			{ID: "ui-backup", Label: "UI Backup", Provider: "anthropic", Model: "claude-3-7-sonnet-latest", SourceID: "anthropic-default"},
		},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)

	body := []byte(`{"experts":[{"id":"ui-ux-expert","label":"UI/UX 专家","description":"负责界面结构与交互设计","category":"design","primary_model_id":"ui-primary","secondary_model_id":"ui-backup","fallback_on":["request_error"],"enabled_skills":["ui-ux-pro-max"],"system_prompt":"你是一名 UI/UX 专家。","prompt_template":"{{prompt}}","output_format":"目标 -> 结构 -> 建议","max_output_tokens":4096,"timeout_ms":45000,"enabled":true}]}`)
	res, err := http.DefaultClient.Do(&http.Request{Method: http.MethodPut, URL: mustParseURL(t, env.httpSrv.URL+"/api/v1/settings/experts"), Header: http.Header{"Content-Type": []string{"application/json"}}, Body: ioNopCloser(bytes.NewReader(body))})
	if err != nil {
		t.Fatalf("put experts: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}

	publicRes, err := http.Get(env.httpSrv.URL + "/api/v1/experts")
	if err != nil {
		t.Fatalf("get experts: %v", err)
	}
	defer publicRes.Body.Close()
	if publicRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", publicRes.Status)
	}
	var experts []struct {
		ID             string `json:"id"`
		PrimaryModelID string `json:"primary_model_id"`
	}
	if err := json.NewDecoder(publicRes.Body).Decode(&experts); err != nil {
		t.Fatalf("decode experts: %v", err)
	}
	found := false
	for _, expert := range experts {
		if expert.ID == "ui-ux-expert" {
			found = true
			if expert.PrimaryModelID != "ui-primary" {
				t.Fatalf("unexpected primary_model_id: %q", expert.PrimaryModelID)
			}
		}
	}
	if !found {
		t.Fatalf("custom expert not found in public list")
	}
}

func TestExpertSettings_GenerateWithDemoBuilder(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai", APIKey: "test-openai"}},
		Models:  []config.LLMModelConfig{{ID: "design-main", Label: "Design Main", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"}},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	body := []byte(`{"builder_expert_id":"demo","messages":[{"role":"user","content":"帮我创建一个偏 B 端后台的 UI 专家"}]}`)
	res, err := http.DefaultClient.Do(&http.Request{Method: http.MethodPost, URL: mustParseURL(t, env.httpSrv.URL+"/api/v1/settings/experts/generate"), Header: http.Header{"Content-Type": []string{"application/json"}}, Body: ioNopCloser(bytes.NewReader(body))})
	if err != nil {
		t.Fatalf("generate expert: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	var out struct {
		AssistantMessage string `json:"assistant_message"`
		Draft            struct {
			ID             string `json:"id"`
			PrimaryModelID string `json:"primary_model_id"`
			Enabled        bool   `json:"enabled"`
		} `json:"draft"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode generate response: %v", err)
	}
	if out.AssistantMessage == "" {
		t.Fatalf("assistant_message is empty")
	}
	if out.Draft.ID == "" {
		t.Fatalf("draft id is empty")
	}
	if out.Draft.PrimaryModelID != "design-main" {
		t.Fatalf("unexpected primary model: %q", out.Draft.PrimaryModelID)
	}
	if !out.Draft.Enabled {
		t.Fatalf("expected generated draft to be enabled")
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return u
}

type nopCloser struct{ *bytes.Reader }

func (n nopCloser) Close() error { return nil }

func ioNopCloser(r *bytes.Reader) nopCloser { return nopCloser{Reader: r} }
