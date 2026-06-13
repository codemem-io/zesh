package lsp

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestTransportRoundTrip(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","method":"test"}`)

	var buf bytes.Buffer
	tr := newTransport(&buf, &buf)

	if err := tr.writeMessage(payload); err != nil {
		t.Fatalf("writeMessage: %v", err)
	}

	got, err := tr.readMessage()
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestTransportMultipleMessages(t *testing.T) {
	messages := [][]byte{
		[]byte(`{"id":1}`),
		[]byte(`{"id":2,"result":null}`),
		[]byte(`{"method":"notify"}`),
	}

	var buf bytes.Buffer
	tr := newTransport(&buf, &buf)

	for _, msg := range messages {
		if err := tr.writeMessage(msg); err != nil {
			t.Fatalf("writeMessage: %v", err)
		}
	}

	for i, want := range messages {
		got, err := tr.readMessage()
		if err != nil {
			t.Fatalf("readMessage[%d]: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("message[%d]: got %q, want %q", i, got, want)
		}
	}
}

func TestTransportContentLengthFormat(t *testing.T) {
	payload := []byte(`{}`)

	var buf bytes.Buffer
	tr := newTransport(bytes.NewReader(nil), &buf)
	_ = tr.writeMessage(payload)

	written := buf.String()
	expected := fmt.Sprintf("Content-Length: %d\r\n\r\n{}", len(payload))
	if written != expected {
		t.Errorf("got %q, want %q", written, expected)
	}
}

func TestTransportMissingContentLength(t *testing.T) {
	input := bytes.NewBufferString("Content-Type: application/json\r\n\r\n{}")
	tr := newTransport(input, io.Discard)
	_, err := tr.readMessage()
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}
