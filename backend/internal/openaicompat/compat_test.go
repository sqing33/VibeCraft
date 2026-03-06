package openaicompat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeTextAPIStyle_FallsBackToChatCompletions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found","type":"bad_response_status_code","param":"","code":"bad_response_status_code"}`))
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":null}],\"usage\":null}\n\n")
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	style, out, err := ProbeTextAPIStyle(context.Background(), TextRequest{Model: "gpt", BaseURL: ts.URL, APIKey: "sk-test", Prompt: "ok"})
	if err != nil {
		t.Fatalf("ProbeTextAPIStyle: %v", err)
	}
	if style != APIStyleChatCompletions {
		t.Fatalf("expected chat_completions, got %q", style)
	}
	if strings.TrimSpace(out) != "OK" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProbeTextAPIStyle_PrefersResponsesWhenAvailable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n")
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"CHAT\"},\"finish_reason\":null}],\"usage\":null}\n\n")
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	style, out, err := ProbeTextAPIStyle(context.Background(), TextRequest{Model: "gpt", BaseURL: ts.URL, APIKey: "sk-test", Prompt: "ok"})
	if err != nil {
		t.Fatalf("ProbeTextAPIStyle: %v", err)
	}
	if style != APIStyleResponses {
		t.Fatalf("expected responses, got %q", style)
	}
	if strings.TrimSpace(out) != "OK" {
		t.Fatalf("unexpected output: %q", out)
	}
}
