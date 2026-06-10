package output

import (
	"fmt"
	"strings"
	"time"
	"zesh/internal/mapfile"
)

// PrintTree prints the node tree in a human-readable indented format.
// When plain is true, only names are shown. Otherwise descriptions and language are shown.
func PrintTree(nodes []*mapfile.Node, plain bool) {
	printNodes(nodes, "", plain)
}

func printNodes(nodes []*mapfile.Node, prefix string, plain bool) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		line := prefix + connector + node.Name
		if !plain && node.Type == "file" {
			if node.Language != "" {
				line += fmt.Sprintf(" [%s]", node.Language)
			}
			if node.Description != "" {
				line += " — " + node.Description
			}
			if len(node.Tags) > 0 {
				line += " " + formatTags(node.Tags)
			}
		}
		fmt.Println(line)

		if len(node.Children) > 0 {
			printNodes(node.Children, childPrefix, plain)
		}
	}
}

// PrintLLM outputs a flat, token-efficient format for LLM context.
// Directories are omitted; only files are listed with their full path.
func PrintLLM(nodes []*mapfile.Node) {
	fmt.Printf("# zesh map — %s\n", time.Now().UTC().Format("2006-01-02"))
	printLLMNodes(nodes)
}

func printLLMNodes(nodes []*mapfile.Node) {
	for _, node := range nodes {
		if node.Type == "file" {
			parts := []string{node.Path}
			if node.Language != "" {
				parts = append(parts, node.Language)
			}
			if node.Description != "" {
				parts = append(parts, node.Description)
			}
			if len(node.Tags) > 0 {
				parts = append(parts, formatTags(node.Tags))
			}
			fmt.Println(strings.Join(parts, " | "))
		}
		if len(node.Children) > 0 {
			printLLMNodes(node.Children)
		}
	}
}

// PrintInfo prints all fields of a single node (no children).
func PrintInfo(node *mapfile.Node) {
	fmt.Printf("path:     %s\n", node.Path)
	fmt.Printf("type:     %s\n", node.Type)
	if node.Language != "" {
		fmt.Printf("language: %s\n", node.Language)
	}
	if node.SizeBytes > 0 {
		fmt.Printf("size:     %d bytes\n", node.SizeBytes)
	}
	if node.LastModified != nil {
		fmt.Printf("modified: %s\n", node.LastModified.Format(time.RFC3339))
	}
	if node.Description != "" {
		fmt.Printf("desc:     %s\n", node.Description)
	}
	if len(node.Tags) > 0 {
		fmt.Printf("tags:     %s\n", strings.Join(node.Tags, ", "))
	}
}

func formatTags(tags []string) string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = "#" + t
	}
	return strings.Join(out, " ")
}
