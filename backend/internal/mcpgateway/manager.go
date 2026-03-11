package mcpgateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"vibe-tree/backend/internal/config"
)

const managedGatewayServerID = "vibe_tree_gateway"

type ConnectionInfo struct {
	ServerID string
	URL      string
	Token    string
	Headers  map[string]string
	Signature string
}

type Status struct {
	Enabled        bool              `json:"enabled"`
	Reachable      bool              `json:"reachable"`
	IdleTTLSeconds int               `json:"idle_ttl_seconds"`
	Sessions       int               `json:"sessions"`
	Downstreams    []DownstreamState `json:"downstreams"`
}

type DownstreamState struct {
	WorkspacePath string `json:"workspace_path"`
	ServerID      string `json:"server_id"`
	Running       bool   `json:"running"`
	LastUsedAt    int64  `json:"last_used_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

type Manager struct {
	baseURL string

	mu            sync.RWMutex
	enabled       bool
	idleTTL       time.Duration
	handler       http.Handler
	sessionByID   map[string]*sessionAccess
	sessionByToken map[string]*sessionAccess
	downstreams   map[string]*downstreamRuntime
	stopCh        chan struct{}
	doneCh        chan struct{}
}

type sessionAccess struct {
	sessionID     string
	workspacePath string
	signature     string
	token         string
	allowedIDs    []string
	server        *mcp.Server
	tools         map[string]toolRoute
}

type toolRoute struct {
	PublicName   string
	ServerID     string
	OriginalName string
	Tool         *mcp.Tool
}

type downstreamRuntime struct {
	key           string
	workspacePath string
	serverID      string

	mu       sync.Mutex
	config   map[string]any
	client   *mcp.Client
	session  *mcp.ClientSession
	lastUsed time.Time
	lastErr  string
}

func New(baseURL string, cfg config.Config) *Manager {
	manager := &Manager{
		baseURL:         strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		sessionByID:     make(map[string]*sessionAccess),
		sessionByToken:  make(map[string]*sessionAccess),
		downstreams:     make(map[string]*downstreamRuntime),
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}
	inner := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		token := bearerToken(req.Header.Get("Authorization"))
		manager.mu.RLock()
		defer manager.mu.RUnlock()
		if state := manager.sessionByToken[token]; state != nil {
			return state.server
		}
		return nil
	}, nil)
	manager.handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		manager.serveHTTP(inner, w, req)
	})
	manager.ReloadConfig(cfg)
	go manager.reapLoop()
	return manager
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	<-m.doneCh
	m.mu.Lock()
	runtimes := make([]*downstreamRuntime, 0, len(m.downstreams))
	for _, rt := range m.downstreams {
		runtimes = append(runtimes, rt)
	}
	m.downstreams = make(map[string]*downstreamRuntime)
	m.sessionByID = make(map[string]*sessionAccess)
	m.sessionByToken = make(map[string]*sessionAccess)
	m.mu.Unlock()
	var firstErr error
	for _, rt := range runtimes {
		if err := rt.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) ReloadConfig(cfg config.Config) {
	if m == nil {
		return
	}
	config.NormalizeMCPGatewaySettings(&cfg.MCPGateway)
	m.mu.Lock()
	m.enabled = cfg.MCPGateway != nil && cfg.MCPGateway.Enabled
	if cfg.MCPGateway != nil {
		m.idleTTL = time.Duration(cfg.MCPGateway.IdleTTLSeconds) * time.Second
	} else {
		m.idleTTL = 10 * time.Minute
	}
	disable := !m.enabled
	if disable {
		m.sessionByID = make(map[string]*sessionAccess)
		m.sessionByToken = make(map[string]*sessionAccess)
	}
	runtimes := make([]*downstreamRuntime, 0)
	if disable {
		for _, rt := range m.downstreams {
			runtimes = append(runtimes, rt)
		}
		m.downstreams = make(map[string]*downstreamRuntime)
	}
	m.mu.Unlock()
	for _, rt := range runtimes {
		_ = rt.close()
	}
}

func (m *Manager) HTTPHandler() http.Handler {
	if m == nil {
		return http.NotFoundHandler()
	}
	return m.handler
}

func (m *Manager) Enabled() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

func (m *Manager) Status() Status {
	status := Status{}
	if m == nil {
		return status
	}
	m.mu.RLock()
	status.Enabled = m.enabled
	status.Reachable = m.enabled
	status.IdleTTLSeconds = int(m.idleTTL / time.Second)
	status.Sessions = len(m.sessionByID)
	runtimes := make([]*downstreamRuntime, 0, len(m.downstreams))
	for _, rt := range m.downstreams {
		runtimes = append(runtimes, rt)
	}
	m.mu.RUnlock()

	states := make([]DownstreamState, 0, len(runtimes))
	for _, rt := range runtimes {
		rt.mu.Lock()
		state := DownstreamState{
			WorkspacePath: rt.workspacePath,
			ServerID:      rt.serverID,
			Running:       rt.session != nil,
			LastError:     strings.TrimSpace(rt.lastErr),
		}
		if !rt.lastUsed.IsZero() {
			state.LastUsedAt = rt.lastUsed.UnixMilli()
		}
		rt.mu.Unlock()
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		if states[i].WorkspacePath == states[j].WorkspacePath {
			return states[i].ServerID < states[j].ServerID
		}
		return states[i].WorkspacePath < states[j].WorkspacePath
	})
	status.Downstreams = states
	return status
}

func (m *Manager) EnsureSessionAccess(ctx context.Context, sessionID, workspacePath string, allowedIDs []string) (*ConnectionInfo, error) {
	if m == nil {
		return nil, nil
	}
	if !m.Enabled() {
		return nil, nil
	}
	sessionID = strings.TrimSpace(sessionID)
	workspacePath = strings.TrimSpace(workspacePath)
	allowedIDs = normalizeIDs(allowedIDs)
	signature := sessionSignature(workspacePath, allowedIDs)

	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return nil, fmt.Errorf("load persisted mcp config: %w", err)
	}
	registry := registryForIDs(cfg.MCPServers, allowedIDs)
	if err := m.ensureSessionTools(ctx, sessionID, workspacePath, signature, allowedIDs, registry); err != nil {
		return nil, err
	}
	token, headers := m.currentTokenForSession(sessionID)
	return &ConnectionInfo{
		ServerID:  managedGatewayServerID,
		URL:       m.baseURL + "/mcp",
		Token:     token,
		Headers:   headers,
		Signature: signature,
	}, nil
}

func (c *ConnectionInfo) CodexServerConfig() map[string]any {
	if c == nil {
		return nil
	}
	// Codex 的 streamable HTTP MCP 配置支持通过环境变量注入 bearer token
	// （参见 `codex mcp add --url ... --bearer-token-env-var ...`）。
	//
	// vibe-tree 在启动每个会话的 Codex app-server 时，会把 token 写入该 env var，
	// 从而避免在 config payload 中直接下发 header/token。
	return map[string]any{
		"type":    "http",
		"url":     c.URL,
		"bearer_token_env_var": "VIBE_TREE_MCP_GATEWAY_TOKEN",
	}
}

func (c *ConnectionInfo) ClaudeMCPConfig() map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"type":    "http",
		"url":     c.URL,
		"headers": cloneStringMap(c.Headers),
	}
}

func (c *ConnectionInfo) OpenCodeMCPConfig() map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"type":    "remote",
		"url":     c.URL,
		"headers": cloneStringMap(c.Headers),
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (m *Manager) currentTokenForSession(sessionID string) (string, map[string]string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if state := m.sessionByID[strings.TrimSpace(sessionID)]; state != nil {
		return state.token, map[string]string{"Authorization": "Bearer " + state.token}
	}
	return "", nil
}

func (m *Manager) ensureSessionTools(ctx context.Context, sessionID, workspacePath, signature string, allowedIDs []string, registry map[string]map[string]any) error {
	m.mu.Lock()
	state := m.sessionByID[sessionID]
	if state == nil {
		state = &sessionAccess{
			sessionID:     sessionID,
			workspacePath: workspacePath,
			allowedIDs:    append([]string(nil), allowedIDs...),
			token:         randomToken(),
			signature:     signature,
			server:        mcp.NewServer(&mcp.Implementation{Name: managedGatewayServerID, Version: "v1"}, nil),
			tools:         make(map[string]toolRoute),
		}
		m.sessionByID[sessionID] = state
		m.sessionByToken[state.token] = state
	} else if state.signature != signature {
		delete(m.sessionByToken, state.token)
		state.workspacePath = workspacePath
		state.allowedIDs = append([]string(nil), allowedIDs...)
		state.signature = signature
		state.token = randomToken()
		m.sessionByToken[state.token] = state
	}
	m.mu.Unlock()

	nextTools := make(map[string]toolRoute)
	for _, serverID := range allowedIDs {
		cfgMap, ok := registry[serverID]
		if !ok {
			continue
		}
		tools, err := m.loadToolCatalog(ctx, workspacePath, serverID, cfgMap)
		if err != nil {
			continue
		}
		for _, item := range tools {
			nextTools[item.PublicName] = item
		}
	}

	existing := make(map[string]toolRoute, len(state.tools))
	for key, value := range state.tools {
		existing[key] = value
	}
	for publicName := range existing {
		if _, ok := nextTools[publicName]; ok {
			continue
		}
		state.server.RemoveTools(publicName)
		delete(state.tools, publicName)
	}
	for publicName, route := range nextTools {
		existingRoute, ok := state.tools[publicName]
		if ok && toolEquivalent(existingRoute.Tool, route.Tool) {
			state.tools[publicName] = route
			continue
		}
		if ok {
			state.server.RemoveTools(publicName)
		}
		routeCopy := route
		state.server.AddTool(cloneTool(route.Tool), func(callCtx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return m.forwardToolCall(callCtx, state.workspacePath, routeCopy, req)
		})
		state.tools[publicName] = route
	}
	return nil
}

func (m *Manager) loadToolCatalog(ctx context.Context, workspacePath, serverID string, cfgMap map[string]any) ([]toolRoute, error) {
	rt, err := m.ensureRuntime(ctx, workspacePath, serverID, cfgMap)
	if err != nil {
		return nil, err
	}
	result, err := rt.listTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]toolRoute, 0, len(result.Tools))
	for _, item := range result.Tools {
		originalName := item.Name
		publicName := publicToolName(serverID, originalName)
		toolCopy := *item
		toolCopy.Name = publicName
		if strings.TrimSpace(toolCopy.Title) == "" {
			toolCopy.Title = publicName
		}
		out = append(out, toolRoute{
			PublicName:   publicName,
			ServerID:     serverID,
			OriginalName: originalName,
			Tool:         &toolCopy,
		})
	}
	return out, nil
}

func (m *Manager) forwardToolCall(ctx context.Context, workspacePath string, route toolRoute, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return nil, err
	}
	registry := registryForIDs(cfg.MCPServers, []string{route.ServerID})
	cfgMap, ok := registry[route.ServerID]
	if !ok {
		return nil, fmt.Errorf("mcp server %q not found", route.ServerID)
	}
	rt, err := m.ensureRuntime(ctx, workspacePath, route.ServerID, cfgMap)
	if err != nil {
		return nil, err
	}
	args := map[string]any{}
	if req != nil && req.Params != nil && req.Params.Arguments != nil {
		_ = json.Unmarshal(req.Params.Arguments, &args)
	}
	return rt.callTool(ctx, route.OriginalName, args)
}

func (m *Manager) ensureRuntime(ctx context.Context, workspacePath, serverID string, cfgMap map[string]any) (*downstreamRuntime, error) {
	key := workspacePath + "\x00" + serverID
	m.mu.Lock()
	rt := m.downstreams[key]
	if rt == nil {
		rt = &downstreamRuntime{key: key, workspacePath: workspacePath, serverID: serverID}
		m.downstreams[key] = rt
	}
	m.mu.Unlock()
	if err := rt.ensureConnected(ctx, cloneJSONMap(cfgMap), m.idleTTL); err != nil {
		return nil, err
	}
	return rt, nil
}

func (m *Manager) serveHTTP(inner http.Handler, w http.ResponseWriter, req *http.Request) {
	if !m.Enabled() {
		http.Error(w, "mcp gateway disabled", http.StatusNotFound)
		return
	}
	token := bearerToken(req.Header.Get("Authorization"))
	if token == "" {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}
	m.mu.RLock()
	state := m.sessionByToken[token]
	m.mu.RUnlock()
	if state == nil {
		http.Error(w, "invalid authorization", http.StatusUnauthorized)
		return
	}
	inner.ServeHTTP(w, req)
}

func (m *Manager) reapLoop() {
	ticker := time.NewTicker(time.Minute)
	defer func() {
		ticker.Stop()
		close(m.doneCh)
	}()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.reapIdleDownstreams()
		}
	}
}

func (m *Manager) reapIdleDownstreams() {
	if m == nil || m.idleTTL <= 0 {
		return
	}
	m.mu.RLock()
	runtimes := make([]*downstreamRuntime, 0, len(m.downstreams))
	for _, rt := range m.downstreams {
		runtimes = append(runtimes, rt)
	}
	m.mu.RUnlock()
	for _, rt := range runtimes {
		rt.mu.Lock()
		expired := rt.session != nil && !rt.lastUsed.IsZero() && time.Since(rt.lastUsed) > m.idleTTL
		rt.mu.Unlock()
		if !expired {
			continue
		}
		_ = rt.close()
	}
}

func normalizeIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sessionSignature(workspacePath string, allowedIDs []string) string {
	payload, _ := json.Marshal(struct {
		WorkspacePath string   `json:"workspace_path"`
		AllowedIDs    []string `json:"allowed_ids"`
	}{WorkspacePath: strings.TrimSpace(workspacePath), AllowedIDs: normalizeIDs(allowedIDs)})
	return string(payload)
}

func registryForIDs(servers []config.MCPServerConfig, ids []string) map[string]map[string]any {
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[strings.TrimSpace(id)] = struct{}{}
	}
	out := make(map[string]map[string]any)
	for _, server := range servers {
		if _, ok := selected[strings.TrimSpace(server.ID)]; !ok {
			continue
		}
		out[strings.TrimSpace(server.ID)] = cloneJSONMap(server.Config)
	}
	return out
}

func cloneTool(tool *mcp.Tool) *mcp.Tool {
	if tool == nil {
		return nil
	}
	copied := *tool
	return &copied
}

func toolEquivalent(a, b *mcp.Tool) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func publicToolName(serverID, toolName string) string {
	serverID = sanitizeName(serverID)
	toolName = sanitizeName(toolName)
	name := serverID + "." + toolName
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "tool"
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('-')
	}
	out := strings.Trim(builder.String(), "-")
	if out == "" {
		return "tool"
	}
	return out
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

func randomToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("vibe-tree-%d", time.Now().UnixNano())
	}
	return "vibe-tree-" + hex.EncodeToString(buf)
}

func cloneJSONMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (rt *downstreamRuntime) ensureConnected(ctx context.Context, cfgMap map[string]any, idleTTL time.Duration) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if idleTTL > 0 && rt.session != nil && !rt.lastUsed.IsZero() && time.Since(rt.lastUsed) > idleTTL {
		_ = rt.closeLocked()
	}
	if rt.session != nil && mapsEqual(rt.config, cfgMap) {
		rt.lastUsed = time.Now()
		return nil
	}
	if err := rt.closeLocked(); err != nil {
		return err
	}
	session, err := connectClientSession(ctx, cfgMap)
	if err != nil {
		rt.lastErr = err.Error()
		return err
	}
	rt.client = mcp.NewClient(&mcp.Implementation{Name: "vibe-tree-gateway", Version: "v1"}, nil)
	rt.session = session
	rt.config = cfgMap
	rt.lastErr = ""
	rt.lastUsed = time.Now()
	return nil
}

func connectClientSession(ctx context.Context, cfgMap map[string]any) (*mcp.ClientSession, error) {
	command := strings.TrimSpace(stringValue(cfgMap["command"]))
	if command != "" {
		args := stringSlice(cfgMap["args"])
		cmd := exec.Command(command, args...)
		cmd.Env = mergeEnv(os.Environ(), stringMap(cfgMap["env"]))
		transport := &mcp.CommandTransport{Command: cmd}
		client := mcp.NewClient(&mcp.Implementation{Name: "vibe-tree-gateway", Version: "v1"}, nil)
		return client.Connect(ctx, transport, nil)
	}
	url := strings.TrimSpace(firstNonEmpty(stringValue(cfgMap["url"]), stringValue(cfgMap["httpUrl"])))
	if url == "" {
		return nil, errors.New("mcp config missing command or url")
	}
	httpClient := &http.Client{Transport: &headerRoundTripper{base: http.DefaultTransport, headers: gatewayHeadersForConfig(cfgMap)}}
	transport := &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "vibe-tree-gateway", Version: "v1"}, nil)
	return client.Connect(ctx, transport, nil)
}

func (rt *downstreamRuntime) listTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.session == nil {
		return nil, errors.New("downstream session not ready")
	}
	rt.lastUsed = time.Now()
	result, err := rt.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		rt.lastErr = err.Error()
		return nil, err
	}
	rt.lastErr = ""
	return result, nil
}

func (rt *downstreamRuntime) callTool(ctx context.Context, toolName string, args map[string]any) (*mcp.CallToolResult, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.session == nil {
		return nil, errors.New("downstream session not ready")
	}
	rt.lastUsed = time.Now()
	result, err := rt.session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		rt.lastErr = err.Error()
		return nil, err
	}
	rt.lastErr = ""
	return result, nil
}

func (rt *downstreamRuntime) close() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.closeLocked()
}

func (rt *downstreamRuntime) closeLocked() error {
	if rt.session != nil {
		_ = rt.session.Close()
	}
	rt.session = nil
	rt.client = nil
	rt.config = nil
	rt.lastUsed = time.Time{}
	return nil
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (rt *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	for key, value := range rt.headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		cloned.Header.Set(key, value)
	}
	return base.RoundTrip(cloned)
}

func gatewayHeadersForConfig(cfgMap map[string]any) map[string]string {
	headers := stringMap(cfgMap["headers"])
	if token := strings.TrimSpace(stringValue(cfgMap["bearer_token"])); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	if envName := strings.TrimSpace(stringValue(cfgMap["bearer_token_env_var"])); envName != "" {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			headers["Authorization"] = "Bearer " + value
		}
	}
	return headers
}

func stringValue(value any) string {
	switch vv := value.(type) {
	case string:
		return vv
	default:
		return ""
	}
}

func stringSlice(value any) []string {
	switch vv := value.(type) {
	case []string:
		return append([]string(nil), vv...)
	case []any:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func stringMap(value any) map[string]string {
	out := map[string]string{}
	switch vv := value.(type) {
	case map[string]string:
		for key, item := range vv {
			if strings.TrimSpace(key) != "" {
				out[key] = item
			}
		}
	case map[string]any:
		for key, item := range vv {
			if text, ok := item.(string); ok && strings.TrimSpace(key) != "" {
				out[key] = text
			}
		}
	}
	return out
}

func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	envMap := make(map[string]string, len(base)+len(overrides))
	for _, item := range base {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envMap[parts[0]] = parts[1]
	}
	for key, value := range overrides {
		envMap[key] = value
	}
	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+envMap[key])
	}
	return out
}

func mapsEqual(a, b map[string]any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
