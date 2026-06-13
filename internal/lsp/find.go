package lsp

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SymbolMatch is a single result from Find.
type SymbolMatch struct {
	Name string
	Kind string
	File string
	Line int
}

// Find searches for symbols whose names contain query (case-insensitive).
// searchPath may be a file, a directory, or empty (whole workspace).
func Find(ctx context.Context, query, searchPath string) ([]SymbolMatch, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get cwd: %w", err)
	}

	if searchPath == "" {
		return findInWorkspace(ctx, query, findWorkspaceRoot(cwd))
	}

	abs, err := filepath.Abs(searchPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	if !info.IsDir() {
		return findInFile(ctx, query, abs)
	}
	return findInWorkspace(ctx, query, abs)
}

func findInFile(ctx context.Context, query, absFile string) ([]SymbolMatch, error) {
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

	if _, err = client.Initialize(ctx, InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   rootURI,
		Capabilities: map[string]any{
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{
					"hierarchicalDocumentSymbolSupport": true,
				},
			},
		},
	}); err != nil {
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

	symbols, err := client.DocumentSymbol(ctx, DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI},
	})
	if err != nil {
		return nil, fmt.Errorf("LSP documentSymbol: %w", err)
	}

	if len(symbols) == 0 {
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

	var results []SymbolMatch
	lower := strings.ToLower(query)
	collectMatches(symbols, lower, absFile, &results)
	return results, nil
}

func findInWorkspace(ctx context.Context, query, root string) ([]SymbolMatch, error) {
	sentinel, err := findSourceFile(root)
	if err != nil {
		return nil, err
	}

	client, closeFn, err := Start(ctx, sentinel)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	rootURI, err := workspaceURI(sentinel)
	if err != nil {
		return nil, err
	}

	if _, err = client.Initialize(ctx, InitializeParams{
		ProcessID:    os.Getpid(),
		RootURI:      rootURI,
		Capabilities: map[string]any{},
	}); err != nil {
		return nil, fmt.Errorf("LSP initialize: %w", err)
	}

	if err := client.Initialized(ctx); err != nil {
		return nil, fmt.Errorf("LSP initialized: %w", err)
	}

	// Give the LSP server a moment to begin indexing.
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	infos, err := client.WorkspaceSymbols(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("workspace/symbol: %w", err)
	}

	var results []SymbolMatch
	for _, info := range infos {
		filePath := uriToPath(info.Location.URI)
		if strings.Contains(filePath, "/vendor/") {
			continue
		}
		// When root is a subdirectory, filter to that subtree.
		if !strings.HasPrefix(filePath, root) {
			continue
		}
		results = append(results, SymbolMatch{
			Name: info.Name,
			Kind: symbolKindName(info.Kind),
			File: filePath,
			Line: info.Location.Range.Start.Line + 1,
		})
	}
	return results, nil
}

// collectMatches recursively filters document symbols by case-insensitive query.
func collectMatches(symbols []DocumentSymbol, lowerQuery, file string, out *[]SymbolMatch) {
	for _, s := range symbols {
		if strings.Contains(strings.ToLower(s.Name), lowerQuery) {
			*out = append(*out, SymbolMatch{
				Name: s.Name,
				Kind: symbolKindName(s.Kind),
				File: file,
				Line: s.Range.Start.Line + 1,
			})
		}
		collectMatches(s.Children, lowerQuery, file, out)
	}
}

// findSourceFile returns the first supported source file found under root.
func findSourceFile(root string) (string, error) {
	for _, ext := range []string{".go", ".ts", ".py", ".rs"} {
		var found string
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if found != "" {
				return filepath.SkipAll
			}
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.Contains(path, "/vendor/") || strings.Contains(path, "/.git/") {
				return nil
			}
			if filepath.Ext(path) == ext {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if found != "" {
			return found, nil
		}
	}
	return "", fmt.Errorf("no supported source files found in %s", root)
}

func symbolKindName(kind int) string {
	switch kind {
	case 1:
		return "file"
	case 2:
		return "module"
	case 3:
		return "namespace"
	case 4:
		return "package"
	case 5:
		return "class"
	case 6:
		return "method"
	case 7:
		return "property"
	case 8:
		return "field"
	case 9:
		return "constructor"
	case 10:
		return "enum"
	case 11:
		return "interface"
	case 12:
		return "function"
	case 13:
		return "variable"
	case 14:
		return "constant"
	case 23:
		return "struct"
	case 26:
		return "type-param"
	default:
		return "symbol"
	}
}
