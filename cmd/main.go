package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Node struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Children []*Node `json:"children,omitempty"`
}

func loadIgnorePatterns(root string) ([]string, error) {
	var patterns []string
	for _, name := range []string{".zeshignore", ".gitignore"} {
		path := filepath.Join(root, name)
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", name, err)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, strings.TrimSuffix(line, "/"))
		}
		f.Close()
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", name, err)
		}
		break
	}
	// append .git by default
	patterns = append(patterns, ".git")
	return patterns, nil
}

func isIgnored(name string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func buildTree(root string, ignore []string) ([]*Node, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", root, err)
	}

	var nodes []*Node
	for _, entry := range entries {
		if isIgnored(entry.Name(), ignore) {
			continue
		}
		node := &Node{
			Name: entry.Name(),
		}
		if entry.IsDir() {
			node.Type = "directory"
			children, err := buildTree(filepath.Join(root, entry.Name()), ignore)
			if err != nil {
				return nil, err
			}
			node.Children = children
		} else {
			node.Type = "file"
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func cmdInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ignorePatterns, err := loadIgnorePatterns(cwd)
	if err != nil {
		return err
	}

	tree, err := buildTree(cwd, ignorePatterns)
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	err = os.MkdirAll("./.zesh/objects", 0777)
	if err != nil {
		return err
	}

	return os.WriteFile("./.zesh/objects/map.json", out, 0644)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: zesh <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		if err := cmdInit(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
