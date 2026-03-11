package mcpstdio

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestReader_ReadMessage_ContentLength(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	input := "Content-Length: " + itoa(len(body)) + "\r\n\r\n" + body
	rd := NewReader(strings.NewReader(input))
	got, framing, err := rd.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != body {
		t.Fatalf("got %q want %q", string(got), body)
	}
	if framing != FramingContentLength {
		t.Fatalf("framing=%v want %v", framing, FramingContentLength)
	}
}

func TestReader_ReadMessage_LineDelimited(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	rd := NewReader(strings.NewReader(body + "\n"))
	got, framing, err := rd.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != body {
		t.Fatalf("got %q want %q", string(got), body)
	}
	if framing != FramingLineDelimited {
		t.Fatalf("framing=%v want %v", framing, FramingLineDelimited)
	}
}

func TestWriteMessage_FramesWithContentLength(t *testing.T) {
	var buf bytes.Buffer
	body := []byte(`{"ok":true}`)
	if err := WriteMessage(&buf, body); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "Content-Length: ") {
		t.Fatalf("missing header: %q", out)
	}
	if !strings.Contains(out, "\r\n\r\n") {
		t.Fatalf("missing header separator: %q", out)
	}
	if !strings.HasSuffix(out, string(body)) {
		t.Fatalf("missing body suffix: %q", out)
	}
}

func itoa(n int) string {
	// Small helper to avoid strconv import in tests.
	if n == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestReader_ReadMessage_EOF(t *testing.T) {
	rd := NewReader(strings.NewReader(""))
	_, _, err := rd.ReadMessage()
	if err != io.EOF {
		t.Fatalf("err=%v want EOF", err)
	}
}
