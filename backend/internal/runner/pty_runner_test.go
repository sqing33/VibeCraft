package runner

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"
)

func TestPTYRunner_StartOneshot_ExitCodeAndOutput(t *testing.T) {
	t.Parallel()

	r := PTYRunner{DefaultGrace: 200 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := r.StartOneshot(ctx, RunSpec{
		Command: "bash",
		Args:    []string{"-lc", `for i in {1..5}; do echo "line:$i"; sleep 0.02; done`},
	})
	if err != nil {
		t.Fatalf("StartOneshot: %v", err)
	}
	defer h.Close()

	var b strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(h.Output())
		for scanner.Scan() {
			b.WriteString(scanner.Text())
			b.WriteString("\n")
		}
	}()

	res, err := h.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	<-done

	out := b.String()
	if !strings.Contains(out, "line:1") || !strings.Contains(out, "line:5") {
		t.Fatalf("unexpected output: %q", out)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (signal=%q)", res.ExitCode, res.Signal)
	}
	if res.StartedAt.IsZero() || res.EndedAt.IsZero() || !res.EndedAt.After(res.StartedAt) {
		t.Fatalf("unexpected timestamps: started=%v ended=%v", res.StartedAt, res.EndedAt)
	}
}

func TestPTYRunner_Cancel(t *testing.T) {
	t.Parallel()

	r := PTYRunner{DefaultGrace: 100 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := r.StartOneshot(ctx, RunSpec{
		Command: "bash",
		Args:    []string{"-lc", `echo "start"; sleep 10; echo "end"`},
	})
	if err != nil {
		t.Fatalf("StartOneshot: %v", err)
	}
	defer h.Close()

	scanner := bufio.NewScanner(h.Output())
	if !scanner.Scan() {
		t.Fatalf("expected output before cancel")
	}
	if !strings.Contains(scanner.Text(), "start") {
		t.Fatalf("unexpected first line: %q", scanner.Text())
	}

	if err := h.Cancel(100 * time.Millisecond); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	res, err := h.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit after cancel, got exit code 0")
	}
	if res.EndedAt.Sub(res.StartedAt) > 3*time.Second {
		t.Fatalf("expected cancel to end quickly, duration=%s", res.EndedAt.Sub(res.StartedAt))
	}
}
