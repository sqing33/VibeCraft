package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestExpertBuilderSession_EndToEnd(t *testing.T) {
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

	createRes := doJSONRequest[struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}](t, http.MethodPost, env.httpSrv.URL+"/api/v1/settings/experts/sessions", map[string]any{
		"title":            "UI 专家设计",
		"builder_model_id": "demo",
	})
	if createRes.Session.ID == "" {
		t.Fatalf("expected session id")
	}

	detail := doJSONRequest[struct {
		Session struct {
			ID               string  `json:"id"`
			TargetExpertID   *string `json:"target_expert_id"`
			LatestSnapshotID *string `json:"latest_snapshot_id"`
		} `json:"session"`
		Messages []struct {
			Role string `json:"role"`
		} `json:"messages"`
		Snapshots []struct {
			ID      string `json:"id"`
			Version int64  `json:"version"`
			Draft   struct {
				ID             string `json:"id"`
				PrimaryModelID string `json:"primary_model_id"`
			} `json:"draft"`
		} `json:"snapshots"`
	}](t, http.MethodPost, env.httpSrv.URL+"/api/v1/settings/experts/sessions/"+createRes.Session.ID+"/messages", map[string]any{
		"content": "我想要一个偏 B 端后台的 UI 专家，重视表格、筛选和页面层级。",
	})
	if len(detail.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(detail.Messages))
	}
	if len(detail.Snapshots) == 0 {
		t.Fatalf("expected snapshots")
	}
	if detail.Snapshots[0].Draft.PrimaryModelID != "design-main" {
		t.Fatalf("unexpected primary_model_id: %q", detail.Snapshots[0].Draft.PrimaryModelID)
	}

	_ = doJSONRequest[struct {
		PublishedExpert struct {
			ID                string `json:"id"`
			BuilderSessionID  string `json:"builder_session_id"`
			BuilderSnapshotID string `json:"builder_snapshot_id"`
		} `json:"published_expert"`
	}](t, http.MethodPost, env.httpSrv.URL+"/api/v1/settings/experts/sessions/"+createRes.Session.ID+"/publish", map[string]any{})

	settingsRes := doJSONRequest[struct {
		Experts []struct {
			ID                string `json:"id"`
			BuilderSessionID  string `json:"builder_session_id"`
			BuilderSnapshotID string `json:"builder_snapshot_id"`
		} `json:"experts"`
	}](t, http.MethodGet, env.httpSrv.URL+"/api/v1/settings/experts", nil)
	found := false
	for _, expert := range settingsRes.Experts {
		if expert.BuilderSessionID == createRes.Session.ID {
			found = true
			if expert.BuilderSnapshotID == "" {
				t.Fatalf("expected builder_snapshot_id")
			}
		}
	}
	if !found {
		t.Fatalf("expected published expert with builder session provenance")
	}
}

func doJSONRequest[T any](t *testing.T, method, url string, body any) T {
	t.Helper()
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %s", res.Status)
	}
	var out T
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}
