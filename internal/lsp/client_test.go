package lsp

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// mockServer writes a canned response for each request it receives.
type mockServer struct {
	conn net.Conn
	t    *transport
}

func newMockServer(t *testing.T, conn net.Conn) *mockServer {
	t.Helper()
	return &mockServer{conn: conn, t: newTransport(conn, conn)}
}

func (m *mockServer) respondTo(result any) {
	raw, _ := m.t.readMessage()
	var req rpcRequest
	_ = json.Unmarshal(raw, &req)

	resultRaw, _ := json.Marshal(result)
	resp, _ := json.Marshal(rpcResponse{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result:  resultRaw,
	})
	_ = m.t.writeMessage(resp)
}

func (m *mockServer) sendServerRequest(method string, id int) {
	req, _ := json.Marshal(struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{Jsonrpc: "2.0", ID: id, Method: method, Params: nil})
	_ = m.t.writeMessage(req)
}

func (m *mockServer) close() { m.conn.Close() }

func newPipe(t *testing.T) (*Client, *mockServer) {
	t.Helper()
	server, client, err := netPipe()
	if err != nil {
		t.Fatalf("netPipe: %v", err)
	}
	c := newClient(client, client)
	s := newMockServer(t, server)
	return c, s
}

func netPipe() (net.Conn, net.Conn, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	defer ln.Close()

	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		c, e := ln.Accept()
		ch <- result{c, e}
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		return nil, nil, err
	}
	res := <-ch
	return res.conn, client, res.err
}

func TestClientInitialize(t *testing.T) {
	client, server := newPipe(t)
	defer server.close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go server.respondTo(InitializeResult{
		Capabilities: map[string]any{"documentSymbolProvider": true},
	})

	result, err := client.Initialize(ctx, InitializeParams{
		ProcessID:    1,
		RootURI:      "file:///tmp",
		Capabilities: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if result.Capabilities == nil {
		t.Error("expected non-nil capabilities")
	}
}

func TestClientServerInitiatedRequest(t *testing.T) {
	client, server := newPipe(t)
	defer server.close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Realistic order: server reads initialize, interjects a server-initiated
	// request, consumes the null reply, then sends the initialize response.
	go func() {
		// Read the initialize request sent by client.
		raw, _ := server.t.readMessage()
		var req rpcRequest
		_ = json.Unmarshal(raw, &req)

		// Send a server-initiated request (gopls does this during handshake).
		server.sendServerRequest("client/registerCapability", 99)
		// Consume the null reply from client.
		server.t.readMessage()

		// Finally send the initialize response.
		resultRaw, _ := json.Marshal(InitializeResult{})
		resp, _ := json.Marshal(rpcResponse{Jsonrpc: "2.0", ID: req.ID, Result: resultRaw})
		_ = server.t.writeMessage(resp)
	}()

	_, err := client.Initialize(ctx, InitializeParams{
		ProcessID:    1,
		RootURI:      "file:///tmp",
		Capabilities: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Initialize after server request: %v", err)
	}
}

func TestClientContextCancellation(t *testing.T) {
	client, server := newPipe(t)
	defer server.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.Initialize(ctx, InitializeParams{})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
