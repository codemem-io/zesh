package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"zesh/internal/lsp"
	"zesh/internal/mapfile"
	"zesh/internal/tree"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"zesh",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	s.AddTool(toolInit(), handleInit)
	s.AddTool(toolMap(), handleMap)
	s.AddTool(toolExport(), handleExport)
	s.AddTool(toolInfo(), handleInfo)
	s.AddTool(toolDescribe(), handleDescribe)
	s.AddTool(toolTag(), handleTag)
	s.AddTool(toolSymbols(), handleSymbols)
	s.AddTool(toolSymbol(), handleSymbol)
	s.AddTool(toolFunction(), handleFunction)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "zesh-mcp error: %v\n", err)
		os.Exit(1)
	}
}

// — tool definitions —

func toolInit() mcp.Tool {
	return mcp.NewTool("init",
		mcp.WithDescription("Scan the current directory and write .zesh/objects/map.json"),
		mcp.WithBoolean("retain", mcp.Description("Preserve existing descriptions and tags")),
	)
}

func toolMap() mcp.Tool {
	return mcp.NewTool("map",
		mcp.WithDescription("Return the codebase node tree from .zesh/objects/map.json"),
	)
}

func toolExport() mcp.Tool {
	return mcp.NewTool("export",
		mcp.WithDescription("Return a flat file list from the map, suitable for LLM context"),
	)
}

func toolInfo() mcp.Tool {
	return mcp.NewTool("info",
		mcp.WithDescription("Return metadata for a specific file or directory path"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Relative path to the file or directory"),
		),
	)
}

func toolDescribe() mcp.Tool {
	return mcp.NewTool("describe",
		mcp.WithDescription("Set or update the description for a file or directory"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Relative path to the file or directory"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Description text to set"),
		),
	)
}

func toolTag() mcp.Tool {
	return mcp.NewTool("tag",
		mcp.WithDescription("Add one or more tags to a file or directory"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Relative path to the file or directory"),
		),
		mcp.WithString("tags",
			mcp.Required(),
			mcp.Description("Comma-separated list of tags to add"),
		),
	)
}

func toolSymbols() mcp.Tool {
	return mcp.NewTool("symbols",
		mcp.WithDescription("Search for symbols matching a keyword. Searches the whole workspace by default, or a specific file/directory when path is given"),
		mcp.WithString("keyword",
			mcp.Required(),
			mcp.Description("Keyword to search for in symbol names"),
		),
		mcp.WithString("path",
			mcp.Description("Optional file or directory path to scope the search"),
		),
	)
}

func toolSymbol() mcp.Tool {
	return mcp.NewTool("symbol",
		mcp.WithDescription("Return full details for a named symbol: source, signature, documentation, and callers"),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("Path to the source file containing the symbol"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Exact symbol name as returned by the symbols tool"),
		),
	)
}

func toolFunction() mcp.Tool {
	return mcp.NewTool("function",
		mcp.WithDescription("Extract a function with LSP-enriched metadata: signature, documentation, source, and callers"),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("Path to the source file"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the function to extract"),
		),
	)
}

// — handlers —

func handleInit(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	retain := req.GetBool("retain", false)

	cwd, err := os.Getwd()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get working directory: %v", err)), nil
	}

	nodes, err := tree.Build(cwd)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if retain {
		old, err := mapfile.Load(mapfile.DefaultPath)
		if err == nil {
			mapfile.Merge(nodes, old)
		}
	}

	if err := mapfile.Save(mapfile.DefaultPath, nodes); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{"ok": true, "path": mapfile.DefaultPath})
}

func handleMap(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	nodes, err := mapfile.Load(mapfile.DefaultPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(nodes)
}

type fileEntry struct {
	Path        string   `json:"path"`
	Language    string   `json:"language,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func handleExport(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	nodes, err := mapfile.Load(mapfile.DefaultPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var files []fileEntry
	collectFiles(nodes, &files)
	return jsonResult(files)
}

func collectFiles(nodes []*mapfile.Node, out *[]fileEntry) {
	for _, n := range nodes {
		if n.Type == "file" {
			*out = append(*out, fileEntry{
				Path:        n.Path,
				Language:    n.Language,
				Description: n.Description,
				Tags:        n.Tags,
			})
		}
		if len(n.Children) > 0 {
			collectFiles(n.Children, out)
		}
	}
}

func handleInfo(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := req.GetString("path", "")
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	nodes, err := mapfile.Load(mapfile.DefaultPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	node := mapfile.FindByPath(nodes, path)
	if node == nil {
		return mcp.NewToolResultError(fmt.Sprintf("path not found in map: %s", path)), nil
	}

	return jsonResult(node)
}

func handleDescribe(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := req.GetString("path", "")
	description := req.GetString("description", "")
	if path == "" || description == "" {
		return mcp.NewToolResultError("path and description are required"), nil
	}

	nodes, err := mapfile.Load(mapfile.DefaultPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	node := mapfile.FindByPath(nodes, path)
	if node == nil {
		return mcp.NewToolResultError(fmt.Sprintf("path not found in map: %s", path)), nil
	}

	node.Description = description

	if err := mapfile.Save(mapfile.DefaultPath, nodes); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{"ok": true, "path": path})
}

func handleTag(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := req.GetString("path", "")
	tagsRaw := req.GetString("tags", "")
	if path == "" || tagsRaw == "" {
		return mcp.NewToolResultError("path and tags are required"), nil
	}

	newTags := strings.Split(tagsRaw, ",")
	for i, t := range newTags {
		newTags[i] = strings.TrimSpace(strings.TrimPrefix(t, "#"))
	}

	nodes, err := mapfile.Load(mapfile.DefaultPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	node := mapfile.FindByPath(nodes, path)
	if node == nil {
		return mcp.NewToolResultError(fmt.Sprintf("path not found in map: %s", path)), nil
	}

	existing := make(map[string]bool, len(node.Tags))
	for _, t := range node.Tags {
		existing[t] = true
	}
	for _, t := range newTags {
		if t != "" && !existing[t] {
			node.Tags = append(node.Tags, t)
			existing[t] = true
		}
	}

	if err := mapfile.Save(mapfile.DefaultPath, nodes); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{"ok": true, "path": path, "tags": node.Tags})
}

func handleSymbols(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	keyword := req.GetString("keyword", "")
	if keyword == "" {
		return mcp.NewToolResultError("keyword is required"), nil
	}
	path := req.GetString("path", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	matches, err := lsp.Find(ctx, keyword, path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(matches)
}

func handleSymbol(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file := req.GetString("file", "")
	name := req.GetString("name", "")
	if file == "" || name == "" {
		return mcp.NewToolResultError("file and name are required"), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := lsp.GetFunction(ctx, file, name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(result)
}

func handleFunction(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file := req.GetString("file", "")
	name := req.GetString("name", "")
	if file == "" || name == "" {
		return mcp.NewToolResultError("file and name are required"), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := lsp.GetFunction(ctx, file, name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(result)
}

// jsonResult marshals v to JSON and wraps it in a text tool result.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
