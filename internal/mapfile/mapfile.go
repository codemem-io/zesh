package mapfile

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const DefaultPath = ".zesh/objects/map.json"

type Node struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Type         string    `json:"type"`
	Description  string    `json:"description,omitempty"`
	Language     string    `json:"language,omitempty"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	LastModified *time.Time `json:"last_modified,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	Children     []*Node   `json:"children,omitempty"`
}

func Load(path string) ([]*Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read map file: %w", err)
	}
	var nodes []*Node
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse map file: %w", err)
	}
	return nodes, nil
}

func Save(path string, nodes []*Node) error {
	if err := os.MkdirAll(".zesh/objects", 0o777); err != nil {
		return fmt.Errorf("failed to create .zesh directory: %w", err)
	}
	out, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal map: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("failed to write map file: %w", err)
	}
	return nil
}

// FindByPath searches nodes recursively for the node with the given path.
func FindByPath(nodes []*Node, path string) *Node {
	for _, n := range nodes {
		if n.Path == path {
			return n
		}
		if len(n.Children) > 0 {
			if found := FindByPath(n.Children, path); found != nil {
				return found
			}
		}
	}
	return nil
}

// Merge copies description and tags from old nodes into new nodes, matched by path.
func Merge(newNodes, oldNodes []*Node) {
	oldIndex := buildIndex(oldNodes)
	applyMeta(newNodes, oldIndex)
}

func buildIndex(nodes []*Node) map[string]*Node {
	index := make(map[string]*Node)
	walkIndex(nodes, index)
	return index
}

func walkIndex(nodes []*Node, index map[string]*Node) {
	for _, n := range nodes {
		index[n.Path] = n
		if len(n.Children) > 0 {
			walkIndex(n.Children, index)
		}
	}
}

func applyMeta(nodes []*Node, index map[string]*Node) {
	for _, n := range nodes {
		if old, ok := index[n.Path]; ok {
			if old.Description != "" {
				n.Description = old.Description
			}
			if len(old.Tags) > 0 {
				n.Tags = old.Tags
			}
		}
		if len(n.Children) > 0 {
			applyMeta(n.Children, index)
		}
	}
}
