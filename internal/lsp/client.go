package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Client dispatches JSON-RPC 2.0 requests over a transport.
type Client struct {
	t       *transport
	mu      sync.Mutex
	nextID  int
	pending map[int]chan rpcResponse
	done    chan struct{}
}

func newClient(r io.Reader, w io.Writer) *Client {
	c := &Client{
		t:       newTransport(r, w),
		pending: make(map[int]chan rpcResponse),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *Client) readLoop() {
	defer close(c.done)
	for {
		raw, err := c.t.readMessage()
		if err != nil {
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = make(map[int]chan rpcResponse)
			c.mu.Unlock()
			return
		}

		// Probe for server-initiated requests (have both method and id).
		var probe struct {
			Method string `json:"method"`
			ID     *int   `json:"id"`
		}
		_ = json.Unmarshal(raw, &probe)

		if probe.Method != "" && probe.ID != nil {
			reply, _ := json.Marshal(rpcResponse{
				Jsonrpc: "2.0",
				ID:      probe.ID,
				Result:  json.RawMessage("null"),
			})
			_ = c.t.writeMessage(reply)
			continue
		}

		var resp rpcResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			continue
		}
		if resp.ID == nil {
			continue // notification
		}

		c.mu.Lock()
		ch, ok := c.pending[*resp.ID]
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
	}
	req, err := json.Marshal(rpcRequest{
		Jsonrpc: "2.0",
		ID:      &id,
		Method:  method,
		Params:  rawParams,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := c.t.writeMessage(req); err != nil {
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("LSP connection closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) notify(method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal notify params: %w", err)
		}
	}
	req, err := json.Marshal(rpcRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  rawParams,
	})
	if err != nil {
		return fmt.Errorf("marshal notify: %w", err)
	}
	return c.t.writeMessage(req)
}

// Typed LSP wrappers.

func (c *Client) Initialize(ctx context.Context, params InitializeParams) (InitializeResult, error) {
	raw, err := c.call(ctx, "initialize", params)
	if err != nil {
		return InitializeResult{}, err
	}
	var result InitializeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return InitializeResult{}, fmt.Errorf("unmarshal InitializeResult: %w", err)
	}
	return result, nil
}

func (c *Client) Initialized(ctx context.Context) error {
	return c.notify("initialized", struct{}{})
}

func (c *Client) DidOpen(ctx context.Context, params DidOpenTextDocumentParams) error {
	return c.notify("textDocument/didOpen", params)
}

func (c *Client) DocumentSymbol(ctx context.Context, params DocumentSymbolParams) ([]DocumentSymbol, error) {
	raw, err := c.call(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}
	var symbols []DocumentSymbol
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, fmt.Errorf("unmarshal DocumentSymbol: %w", err)
	}
	return symbols, nil
}

func (c *Client) Hover(ctx context.Context, params HoverParams) (*Hover, error) {
	raw, err := c.call(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var hover Hover
	if err := json.Unmarshal(raw, &hover); err != nil {
		return nil, fmt.Errorf("unmarshal Hover: %w", err)
	}
	return &hover, nil
}

func (c *Client) References(ctx context.Context, params ReferenceParams) ([]Location, error) {
	raw, err := c.call(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var locations []Location
	if err := json.Unmarshal(raw, &locations); err != nil {
		return nil, fmt.Errorf("unmarshal References: %w", err)
	}
	return locations, nil
}

func (c *Client) WorkspaceSymbols(ctx context.Context, query string) ([]SymbolInformation, error) {
	raw, err := c.call(ctx, "workspace/symbol", WorkspaceSymbolParams{Query: query})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var symbols []SymbolInformation
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, fmt.Errorf("unmarshal WorkspaceSymbols: %w", err)
	}
	return symbols, nil
}

func (c *Client) Shutdown(ctx context.Context) error {
	_, err := c.call(ctx, "shutdown", nil)
	return err
}

func (c *Client) Exit() error {
	return c.notify("exit", nil)
}
