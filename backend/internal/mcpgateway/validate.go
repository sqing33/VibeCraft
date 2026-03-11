package mcpgateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ValidateDownstreamConfig validates that a downstream MCP config is runnable by:
// - connecting and completing initialize (handled by mcp.Client.Connect)
// - listing tools successfully
// - requiring at least one tool (since the current gateway proxies tools only)
func ValidateDownstreamConfig(ctx context.Context, cfgMap map[string]any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	session, closer, err := connectDownstream(ctx, cfgMap)
	if err != nil {
		return err
	}
	defer closer()

	res, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	if res == nil || len(res.Tools) == 0 {
		return errors.New("exposes 0 tools; current vibe-tree gateway proxies tools only")
	}
	return nil
}

func connectDownstream(ctx context.Context, cfgMap map[string]any) (*mcp.ClientSession, func(), error) {
	command := strings.TrimSpace(stringValue(cfgMap["command"]))
	if command != "" {
		args := stringSlice(cfgMap["args"])
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Env = mergeEnv(os.Environ(), stringMap(cfgMap["env"]))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		transport := &mcp.CommandTransport{Command: cmd}
		client := mcp.NewClient(&mcp.Implementation{Name: "vibe-tree-validate", Version: "v1"}, nil)
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				return nil, func() {}, fmt.Errorf("connect stdio: %w (stderr: %s)", err, truncateOneLine(msg, 400))
			}
			return nil, func() {}, fmt.Errorf("connect stdio: %w", err)
		}
		return session, func() { _ = session.Close() }, nil
	}

	url := strings.TrimSpace(firstNonEmpty(stringValue(cfgMap["url"]), stringValue(cfgMap["httpUrl"])))
	if url == "" {
		return nil, func() {}, errors.New("missing command or url")
	}
	httpClient := &http.Client{Transport: &headerRoundTripper{base: http.DefaultTransport, headers: gatewayHeadersForConfig(cfgMap)}}
	transport := &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: httpClient}
	client := mcp.NewClient(&mcp.Implementation{Name: "vibe-tree-validate", Version: "v1"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, func() {}, fmt.Errorf("connect remote: %w", err)
	}
	return session, func() { _ = session.Close() }, nil
}

func truncateOneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

