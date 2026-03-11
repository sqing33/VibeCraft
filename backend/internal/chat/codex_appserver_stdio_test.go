package chat

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCodexAppServerReadLoopIgnoresNonJSONLines(t *testing.T) {
	client := &stdioCodexAppServerClient{
		notifications: make(chan codexAppServerNotification, 8),
		pending:       make(map[string]chan codexRPCEnvelope),
		diagBytes:     make(map[string]int64),
		diagTruncated: make(map[string]bool),
		readDone:      make(chan struct{}),
		waitDone:      make(chan struct{}),
	}

	input := "hello world\n" +
		"{\"method\":\"item/started\",\"params\":{}}\n"
	go client.readLoop(strings.NewReader(input))
	<-client.readDone

	notes := make([]codexAppServerNotification, 0, 2)
	for note := range client.notifications {
		notes = append(notes, note)
	}

	if client.readErr != nil {
		t.Fatalf("expected readErr=nil, got %v", client.readErr)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notes))
	}
	if notes[0].Method != "item/started" {
		t.Fatalf("unexpected method: %q", notes[0].Method)
	}
}

func TestCodexAppServerReadLoopHandlesLargeEnvelope(t *testing.T) {
	client := &stdioCodexAppServerClient{
		notifications: make(chan codexAppServerNotification, 2),
		pending:       make(map[string]chan codexRPCEnvelope),
		diagBytes:     make(map[string]int64),
		diagTruncated: make(map[string]bool),
		readDone:      make(chan struct{}),
		waitDone:      make(chan struct{}),
	}

	delta := strings.Repeat("a", 5*1024*1024)
	env := map[string]any{
		"method": "item/agentMessage/delta",
		"params": map[string]any{"delta": delta},
	}
	line, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	go client.readLoop(strings.NewReader(string(line) + "\n"))
	<-client.readDone

	var note codexAppServerNotification
	ok := false
	for n := range client.notifications {
		note = n
		ok = true
		break
	}
	if !ok {
		t.Fatalf("expected at least one notification")
	}
	if client.readErr != nil {
		t.Fatalf("expected readErr=nil, got %v", client.readErr)
	}
	if note.Method != "item/agentMessage/delta" {
		t.Fatalf("unexpected method: %q", note.Method)
	}
	if len(note.Params) <= 4*1024*1024 {
		t.Fatalf("expected params > 4MB, got %d bytes", len(note.Params))
	}
}

func TestCodexAppServerCallRetriesOverload(t *testing.T) {
	oldSleep := codexAppServerSleep
	defer func() { codexAppServerSleep = oldSleep }()

	sleepCalls := 0
	codexAppServerSleep = func(ctx context.Context, d time.Duration) error {
		sleepCalls++
		return nil
	}

	client := &stdioCodexAppServerClient{
		enc:           json.NewEncoder(io.Discard),
		notifications: make(chan codexAppServerNotification, 1),
		pending:       make(map[string]chan codexRPCEnvelope),
		diagBytes:     make(map[string]int64),
		diagTruncated: make(map[string]bool),
		readDone:      make(chan struct{}),
		waitDone:      make(chan struct{}),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.call(context.Background(), "thread/start", map[string]any{}, nil)
	}()

	overload := &codexRPCError{Code: -32001, Message: "Server overloaded; retry later"}
	for i := 1; i <= 2; i++ {
		key := strconv.Itoa(i)
		respCh := waitPendingRespCh(t, client, key)
		respCh <- codexRPCEnvelope{Error: overload}
	}
	respCh := waitPendingRespCh(t, client, "3")
	respCh <- codexRPCEnvelope{Result: json.RawMessage(`{}`)}

	if err := <-errCh; err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if sleepCalls != 2 {
		t.Fatalf("expected 2 backoff sleeps, got %d", sleepCalls)
	}
}

func waitPendingRespCh(t *testing.T, client *stdioCodexAppServerClient, key string) chan codexRPCEnvelope {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		client.pendingMu.Lock()
		ch := client.pending[key]
		client.pendingMu.Unlock()
		if ch != nil {
			return ch
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for pending response channel key=%q", key)
	return nil
}
