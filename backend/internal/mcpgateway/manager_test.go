package mcpgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"vibecraft/backend/internal/config"
)

func TestManager_EnsureSessionAccessTracksSessions(t *testing.T) {
	cfg := config.Default()
	cfg.MCPGateway = &config.MCPGatewaySettings{Enabled: true, IdleTTLSeconds: 600}
	gateway := New("http://127.0.0.1:7777", cfg)
	defer func() { _ = gateway.Close() }()
	info, err := gateway.EnsureSessionAccess(context.Background(), "sess_1", "/tmp/workspace", []string{})
	if err != nil {
		t.Fatalf("EnsureSessionAccess: %v", err)
	}
	if info == nil || info.Token == "" || info.Headers["Authorization"] == "" {
		t.Fatalf("unexpected connection info: %#v", info)
	}
	status := gateway.Status()
	if !status.Enabled || status.Sessions != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestManager_DisabledRejectsRequests(t *testing.T) {
	gateway := New("http://127.0.0.1:7777", config.Default())
	defer func() { _ = gateway.Close() }()
	srv := httptest.NewServer(gateway.HTTPHandler())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", res.StatusCode)
	}
}
