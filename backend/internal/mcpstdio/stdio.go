package mcpstdio

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Reader accepts both MCP Content-Length framing and a line-delimited JSON fallback.
type Reader struct {
	r *bufio.Reader
}

type Framing int

const (
	FramingUnknown Framing = iota
	FramingContentLength
	FramingLineDelimited
)

func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

func (rd *Reader) ReadMessage() ([]byte, Framing, error) {
	for {
		line, err := rd.r.ReadString('\n')
		if err != nil {
			if err == io.EOF && strings.TrimSpace(line) == "" {
				return nil, FramingUnknown, io.EOF
			}
			// If EOF but we still have bytes, continue processing.
			if err != io.EOF {
				return nil, FramingUnknown, err
			}
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			if err == io.EOF {
				return nil, FramingUnknown, io.EOF
			}
			continue
		}

		if hasContentLengthPrefix(trimmed) {
			n, parseErr := parseContentLength(trimmed)
			if parseErr != nil {
				return nil, FramingUnknown, parseErr
			}
			// Read until the blank line that ends headers.
			for {
				h, herr := rd.r.ReadString('\n')
				if herr != nil {
					return nil, FramingUnknown, herr
				}
				h = strings.TrimRight(h, "\r\n")
				if strings.TrimSpace(h) == "" {
					break
				}
			}
			payload := make([]byte, n)
			if _, err := io.ReadFull(rd.r, payload); err != nil {
				return nil, FramingUnknown, err
			}
			return bytes.TrimSpace(payload), FramingContentLength, nil
		}

		// Fallback: interpret the line as a JSON-RPC message.
		return []byte(strings.TrimSpace(trimmed)), FramingLineDelimited, nil
	}
}

func hasContentLengthPrefix(line string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "content-length:")
}

func parseContentLength(line string) (int, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid Content-Length header")
	}
	v := strings.TrimSpace(parts[1])
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid Content-Length value: %q", v)
	}
	return n, nil
}

// WriteMessage writes Content-Length framed JSON-RPC response.
func WriteMessage(w io.Writer, payload []byte) error {
	payload = bytes.TrimSpace(payload)
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func WriteMessageLine(w io.Writer, payload []byte) error {
	payload = bytes.TrimSpace(payload)
	if _, err := w.Write(payload); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}
