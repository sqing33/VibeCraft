package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"vibecraft/backend/internal/config"
)

func TestLLMTestAPI_DoesNotLeakAPIKey(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)

	apiKey := "sk_secret_abcdef"
	body, _ := json.Marshal(map[string]any{
		"provider": "openai",
		"model":    "gpt-5-codex",
		"api_key":  apiKey,
		"base_url": "http://127.0.0.1:1", // force quick failure
		"prompt":   "ok",
	})

	res, err := http.Post(env.httpSrv.URL+"/api/v1/settings/llm/test", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	if strings.Contains(string(b), apiKey) {
		t.Fatalf("api key leaked in response: %s", string(b))
	}
}
