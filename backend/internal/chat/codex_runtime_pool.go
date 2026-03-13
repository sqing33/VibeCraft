package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"vibecraft/backend/internal/logx"
	"vibecraft/backend/internal/runner"
)

type codexRuntimePool struct {
	idleTTL      time.Duration
	reapInterval time.Duration
	newClient    func(context.Context, runner.RunSpec) (codexAppServerClient, error)
	now          func() time.Time

	mu       sync.Mutex
	sessions map[string]*codexRuntimeEntry
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type codexRuntimeEntry struct {
	sessionID string

	mu        sync.Mutex
	client    codexAppServerClient
	signature string
	threadID  string
	lastUsed  time.Time
}

type codexRuntimeLease struct {
	pool      *codexRuntimePool
	entry     *codexRuntimeEntry
	sessionID string
	fresh     bool
	released  bool
}

func newCodexRuntimePool(idleTTL, reapInterval time.Duration) *codexRuntimePool {
	pool := &codexRuntimePool{
		idleTTL:      idleTTL,
		reapInterval: reapInterval,
		newClient:    newCodexAppServerClient,
		now:          time.Now,
		sessions:     make(map[string]*codexRuntimeEntry),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
	if idleTTL > 0 && reapInterval > 0 {
		go pool.reapLoop()
	} else {
		close(pool.doneCh)
	}
	return pool
}

func (p *codexRuntimePool) Acquire(ctx context.Context, sessionID string, spec runner.RunSpec, req codexAppServerThreadRequest) (*codexRuntimeLease, error) {
	if p == nil {
		return nil, fmt.Errorf("codex runtime pool not configured")
	}
	sessionID = firstNonEmptyTrimmed(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	signature, err := codexRuntimeSignature(req)
	if err != nil {
		return nil, err
	}
	entry := p.getOrCreateEntry(sessionID)
	entry.mu.Lock()
	fresh := false
	if p.runtimeExpiredLocked(entry) || entry.signature != signature {
		if err := p.closeEntryLocked(entry); err != nil {
			entry.mu.Unlock()
			return nil, err
		}
	}
	if entry.client == nil {
		client, err := p.newClient(ctx, spec)
		if err != nil {
			entry.mu.Unlock()
			return nil, err
		}
		if err := client.Initialize(ctx); err != nil {
			_ = client.Close()
			entry.mu.Unlock()
			return nil, err
		}
		entry.client = client
		entry.signature = signature
		entry.threadID = ""
		fresh = true
	}
	entry.lastUsed = p.now()
	return &codexRuntimeLease{pool: p, entry: entry, sessionID: sessionID, fresh: fresh}, nil
}

func (p *codexRuntimePool) Invalidate(sessionID string) error {
	sessionID = firstNonEmptyTrimmed(sessionID)
	if sessionID == "" || p == nil {
		return nil
	}
	p.mu.Lock()
	entry := p.sessions[sessionID]
	if entry != nil {
		delete(p.sessions, sessionID)
	}
	p.mu.Unlock()
	if entry == nil {
		return nil
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return p.closeEntryLocked(entry)
}

func (p *codexRuntimePool) Close() error {
	if p == nil {
		return nil
	}
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
	<-p.doneCh

	p.mu.Lock()
	entries := make([]*codexRuntimeEntry, 0, len(p.sessions))
	for sessionID, entry := range p.sessions {
		delete(p.sessions, sessionID)
		entries = append(entries, entry)
	}
	p.mu.Unlock()

	var firstErr error
	for _, entry := range entries {
		entry.mu.Lock()
		if err := p.closeEntryLocked(entry); err != nil && firstErr == nil {
			firstErr = err
		}
		entry.mu.Unlock()
	}
	return firstErr
}

func (p *codexRuntimePool) getOrCreateEntry(sessionID string) *codexRuntimeEntry {
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry := p.sessions[sessionID]; entry != nil {
		return entry
	}
	entry := &codexRuntimeEntry{sessionID: sessionID}
	p.sessions[sessionID] = entry
	return entry
}

func (p *codexRuntimePool) runtimeExpiredLocked(entry *codexRuntimeEntry) bool {
	if entry == nil || entry.client == nil || p.idleTTL <= 0 || entry.lastUsed.IsZero() {
		return false
	}
	return p.now().Sub(entry.lastUsed) > p.idleTTL
}

func (p *codexRuntimePool) closeEntryLocked(entry *codexRuntimeEntry) error {
	if entry == nil {
		return nil
	}
	if entry.client == nil {
		entry.signature = ""
		entry.threadID = ""
		entry.lastUsed = time.Time{}
		return nil
	}
	client := entry.client
	entry.client = nil
	entry.signature = ""
	entry.threadID = ""
	entry.lastUsed = time.Time{}
	return client.Close()
}

func (p *codexRuntimePool) reapLoop() {
	defer close(p.doneCh)
	ticker := time.NewTicker(p.reapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.reapExpired()
		}
	}
}

func (p *codexRuntimePool) reapExpired() {
	if p == nil || p.idleTTL <= 0 {
		return
	}
	p.mu.Lock()
	entries := make([]*codexRuntimeEntry, 0, len(p.sessions))
	for _, entry := range p.sessions {
		entries = append(entries, entry)
	}
	p.mu.Unlock()
	for _, entry := range entries {
		entry.mu.Lock()
		if !p.runtimeExpiredLocked(entry) {
			entry.mu.Unlock()
			continue
		}
		if err := p.closeEntryLocked(entry); err != nil {
			logx.Warn("chat", "evict-codex-runtime", "回收空闲 Codex 运行时失败", "err", err, "session_id", entry.sessionID)
		}
		entry.mu.Unlock()
		p.mu.Lock()
		if current := p.sessions[entry.sessionID]; current == entry && current.client == nil {
			delete(p.sessions, entry.sessionID)
		}
		p.mu.Unlock()
	}
}

func (l *codexRuntimeLease) Client() codexAppServerClient {
	if l == nil || l.entry == nil {
		return nil
	}
	return l.entry.client
}

func (l *codexRuntimeLease) ThreadID() string {
	if l == nil || l.entry == nil {
		return ""
	}
	return firstNonEmptyTrimmed(l.entry.threadID)
}

func (l *codexRuntimeLease) SetThreadID(threadID string) {
	if l == nil || l.entry == nil {
		return
	}
	l.entry.threadID = firstNonEmptyTrimmed(threadID)
}

func (l *codexRuntimeLease) Fresh() bool {
	if l == nil {
		return false
	}
	return l.fresh
}

func (l *codexRuntimeLease) Discard() error {
	if l == nil || l.entry == nil || l.released {
		return nil
	}
	l.released = true
	defer l.entry.mu.Unlock()
	if l.pool != nil {
		l.pool.mu.Lock()
		if current := l.pool.sessions[l.sessionID]; current == l.entry {
			delete(l.pool.sessions, l.sessionID)
		}
		l.pool.mu.Unlock()
		return l.pool.closeEntryLocked(l.entry)
	}
	return nil
}

func (l *codexRuntimeLease) Release() {
	if l == nil || l.entry == nil || l.released {
		return
	}
	l.released = true
	if l.pool != nil {
		l.entry.lastUsed = l.pool.now()
	}
	l.entry.mu.Unlock()
}

func codexRuntimeSignature(req codexAppServerThreadRequest) (string, error) {
	payload := map[string]any{
		"model":             firstNonEmptyTrimmed(req.Model),
		"cwd":               firstNonEmptyTrimmed(req.Cwd),
		"base_instructions": firstNonEmptyTrimmed(req.BaseInstructions),
		"config":            req.Config,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal codex runtime signature: %w", err)
	}
	return string(buf), nil
}
