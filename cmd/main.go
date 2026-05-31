package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Node struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Children []*Node `json:"children,omitempty"`
}

func buildTree(root string) ([]*Node, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", root, err)
	}

	var nodes []*Node
	for _, entry := range entries {
		node := &Node{
			Name: entry.Name(),
		}
		if entry.IsDir() {
			node.Type = "directory"
			children, err := buildTree(filepath.Join(root, entry.Name()))
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

	tree, err := buildTree(cwd)
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	// fmt.Println(string(out))

	// save the tree in .zesh folder
	err = os.MkdirAll("./.zesh/objects", 0777)
	if err != nil {
		return err
	}

	return os.WriteFile("./.zesh/objects/tree", out, 0777)
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
