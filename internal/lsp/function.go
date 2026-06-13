package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Result holds the extracted function details.
type Result struct {
	Name      string
	Signature string
	Doc       string
	Source    string
	Callers   []Caller
}

type Caller struct {
	File string
	Line int
}

// GetFunction extracts the named function from file using the appropriate LSP server.
func GetFunction(ctx context.Context, file, name string) (*Result, error) {
	absFile, err := filepath.Abs(file)
	if err != nil {
		return nil, fmt.Errorf("resolve file path: %w", err)
	}

	content, err := os.ReadFile(absFile)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	langID, err := LangIDFor(absFile)
	if err != nil {
		return nil, err
	}

	fileURI, err := toFileURI(absFile)
	if err != nil {
		return nil, err
	}

	rootURI, err := workspaceURI(absFile)
	if err != nil {
		return nil, err
	}

	client, closeFn, err := Start(ctx, absFile)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	_, err = client.Initialize(ctx, InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   rootURI,
		Capabilities: map[string]any{
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{
					"hierarchicalDocumentSymbolSupport": true,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LSP initialize: %w", err)
	}

	if err := client.Initialized(ctx); err != nil {
		return nil, fmt.Errorf("LSP initialized: %w", err)
	}

	if err := client.DidOpen(ctx, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        fileURI,
			LanguageID: langID,
			Version:    1,
			Text:       string(content),
		},
	}); err != nil {
		return nil, fmt.Errorf("LSP didOpen: %w", err)
	}

	sym, err := findSymbol(ctx, client, fileURI, name)
	if err != nil {
		return nil, err
	}

	source := extractRange(content, sym.Range)

	hover, _ := client.Hover(ctx, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI},
		Position:     sym.SelectionRange.Start,
	})
	sig, doc := parseHover(hover)

	locations, _ := client.References(ctx, ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI},
		Position:     sym.SelectionRange.Start,
		Context:      ReferenceContext{IncludeDeclaration: false},
	})
	callers := toCallers(locations)

	return &Result{
		Name:      name,
		Signature: sig,
		Doc:       doc,
		Source:    source,
		Callers:   callers,
	}, nil
}

// findSymbol calls documentSymbol and searches for name, retrying once on empty results.
func findSymbol(ctx context.Context, client *Client, fileURI, name string) (*DocumentSymbol, error) {
	symbols, err := client.DocumentSymbol(ctx, DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI},
	})
	if err != nil {
		return nil, fmt.Errorf("LSP documentSymbol: %w", err)
	}

	if len(symbols) == 0 {
		// LSP server may still be indexing — wait and retry once.
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		symbols, err = client.DocumentSymbol(ctx, DocumentSymbolParams{
			TextDocument: TextDocumentIdentifier{URI: fileURI},
		})
		if err != nil {
			return nil, fmt.Errorf("LSP documentSymbol (retry): %w", err)
		}
	}

	sym := searchSymbols(symbols, name)
	if sym == nil {
		return nil, fmt.Errorf("function %q not found in file", name)
	}
	return sym, nil
}

func searchSymbols(symbols []DocumentSymbol, name string) *DocumentSymbol {
	for i := range symbols {
		if symbolNameMatches(symbols[i].Name, name) {
			return &symbols[i]
		}
		if found := searchSymbols(symbols[i].Children, name); found != nil {
			return found
		}
	}
	return nil
}

// symbolNameMatches handles gopls returning method names like "(*Receiver).Method"
// when the caller provides just "Method" or "Receiver.Method".
func symbolNameMatches(symbolName, query string) bool {
	if symbolName == query {
		return true
	}
	// Strip receiver prefix: "(*Foo).Bar" or "(Foo).Bar" → "Bar"
	if idx := strings.LastIndex(symbolName, "."); idx != -1 {
		bare := symbolName[idx+1:]
		if bare == query {
			return true
		}
	}
	return false
}

// extractRange slices content using an LSP Range (0-based, UTF-16 chars).
func extractRange(content []byte, r Range) string {
	start := positionToOffset(content, r.Start)
	end := positionToOffset(content, r.End)
	if start < 0 || end > len(content) || start > end {
		return ""
	}
	return string(content[start:end])
}

func positionToOffset(content []byte, pos Position) int {
	line := 0
	i := 0
	for i < len(content) && line < pos.Line {
		if content[i] == '\n' {
			line++
		}
		i++
	}
	char := 0
	for i < len(content) && char < pos.Character && content[i] != '\n' {
		r, size := utf8.DecodeRune(content[i:])
		if r >= 0x10000 {
			char += 2 // surrogate pair in UTF-16
		} else {
			char++
		}
		i += size
	}
	return i
}

// parseHover extracts the signature and doc string from a Hover response.
// Contents may be a string, MarkupContent, MarkedString, or an array of those.
func parseHover(hover *Hover) (sig, doc string) {
	if hover == nil {
		return "", ""
	}
	text := hoverText(hover.Contents)
	// Separate the first line (signature) from the rest (doc).
	parts := strings.SplitN(strings.TrimSpace(text), "\n", 2)
	sig = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		doc = strings.TrimSpace(parts[1])
	}
	return sig, doc
}

func hoverText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try plain string.
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	// Try MarkupContent / MarkedString object with a "value" field.
	var obj struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Value != "" {
		return obj.Value
	}
	// Try array — use the first element.
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return hoverText(arr[0])
	}
	return ""
}

func toCallers(locations []Location) []Caller {
	callers := make([]Caller, 0, len(locations))
	for _, loc := range locations {
		callers = append(callers, Caller{
			File: uriToPath(loc.URI),
			Line: loc.Range.Start.Line + 1, // convert to 1-based
		})
	}
	return callers
}

func toFileURI(absPath string) (string, error) {
	u := url.URL{Scheme: "file", Path: absPath}
	return u.String(), nil
}

func workspaceURI(absFile string) (string, error) {
	dir := filepath.Dir(absFile)
	root := findWorkspaceRoot(dir)
	u := url.URL{Scheme: "file", Path: root}
	return u.String(), nil
}

// findWorkspaceRoot walks up from dir looking for a module/project root marker.
func findWorkspaceRoot(dir string) string {
	markers := []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml", ".git"}
	cur := dir
	for {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(cur, m)); err == nil {
				return cur
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return dir
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	return u.Path
}
