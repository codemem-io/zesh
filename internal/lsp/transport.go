package lsp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

type transport struct {
	r   *bufio.Reader
	w   io.Writer
	wmu sync.Mutex
}

func newTransport(r io.Reader, w io.Writer) *transport {
	return &transport{r: bufio.NewReader(r), w: w}
}

// writeMessage writes a Content-Length framed message. Safe for concurrent use.
func (t *transport) writeMessage(payload []byte) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := io.WriteString(t.w, header); err != nil {
		return fmt.Errorf("transport write header: %w", err)
	}
	if _, err := t.w.Write(payload); err != nil {
		return fmt.Errorf("transport write body: %w", err)
	}
	return nil
}

// readMessage reads one Content-Length framed message. Not safe for concurrent use.
func (t *transport) readMessage() ([]byte, error) {
	var contentLength int
	for {
		line, err := t.r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("transport read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if v, ok := strings.CutPrefix(line, "Content-Length: "); ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("transport parse Content-Length: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("transport: missing or zero Content-Length")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(t.r, buf); err != nil {
		return nil, fmt.Errorf("transport read body: %w", err)
	}
	return buf, nil
}
