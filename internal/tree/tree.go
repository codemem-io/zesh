package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"zesh/internal/ignore"
	"zesh/internal/mapfile"
)

var extLanguage = map[string]string{
	".go":    "go",
	".ts":    "typescript",
	".tsx":   "typescript",
	".js":    "javascript",
	".jsx":   "javascript",
	".py":    "python",
	".rs":    "rust",
	".java":  "java",
	".kt":    "kotlin",
	".swift": "swift",
	".rb":    "ruby",
	".php":   "php",
	".cs":    "csharp",
	".cpp":   "cpp",
	".c":     "c",
	".h":     "c",
	".sh":    "shell",
	".bash":  "shell",
	".zsh":   "shell",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".toml":  "toml",
	".md":    "markdown",
	".html":  "html",
	".css":   "css",
	".scss":  "scss",
	".sql":   "sql",
	".proto": "protobuf",
}

// Build walks root and returns a tree of nodes with auto-enriched metadata.
func Build(root string) ([]*mapfile.Node, error) {
	patterns, err := ignore.Load(root)
	if err != nil {
		return nil, err
	}
	return walk(root, root, patterns)
}

func walk(root, dir string, patterns []string) ([]*mapfile.Node, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var nodes []*mapfile.Node
	for _, entry := range entries {
		if ignore.Match(entry.Name(), patterns) {
			continue
		}

		relPath, err := filepath.Rel(root, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path: %w", err)
		}
		relPath = filepath.ToSlash(relPath)

		node := &mapfile.Node{
			Name: entry.Name(),
			Path: relPath,
		}

		if entry.IsDir() {
			node.Type = "directory"
			children, err := walk(root, filepath.Join(dir, entry.Name()), patterns)
			if err != nil {
				return nil, err
			}
			node.Children = children
		} else {
			node.Type = "file"
			node.Language = detectLanguage(entry.Name())
			if info, err := entry.Info(); err == nil {
				node.SizeBytes = info.Size()
				mt := info.ModTime().UTC().Truncate(time.Second)
				node.LastModified = &mt
			}
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

func detectLanguage(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	return extLanguage[ext]
}
