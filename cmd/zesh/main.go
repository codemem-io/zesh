package main

import (
	"fmt"
	"os"
	"strings"
	"zesh/internal/mapfile"
	"zesh/internal/output"
	"zesh/internal/tree"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "zesh",
	Short: "zesh — codebase map tool for humans and LLMs",
	Long: `zesh maintains a structured map of your codebase.
It tracks file metadata, descriptions, and tags to make your
project navigable for both humans and LLMs.`,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize or refresh the codebase map",
	Long: `Scans the current directory and writes .zesh/objects/map.json.
Use --retain to preserve existing descriptions and tags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		retain, _ := cmd.Flags().GetBool("retain")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		nodes, err := tree.Build(cwd)
		if err != nil {
			return err
		}

		if retain {
			old, err := mapfile.Load(mapfile.DefaultPath)
			if err == nil {
				mapfile.Merge(nodes, old)
			}
		}

		if err := mapfile.Save(mapfile.DefaultPath, nodes); err != nil {
			return err
		}

		fmt.Println("map written to", mapfile.DefaultPath)
		return nil
	},
}

var describeCmd = &cobra.Command{
	Use:   "describe <path> <description>",
	Short: "Add or update a description for a file or directory",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetPath := args[0]
		description := args[1]

		nodes, err := mapfile.Load(mapfile.DefaultPath)
		if err != nil {
			return err
		}

		node := mapfile.FindByPath(nodes, targetPath)
		if node == nil {
			return fmt.Errorf("path not found in map: %s\nRun 'zesh init' first.", targetPath)
		}

		node.Description = description

		if err := mapfile.Save(mapfile.DefaultPath, nodes); err != nil {
			return err
		}

		fmt.Printf("description set for %s\n", targetPath)
		return nil
	},
}

var tagCmd = &cobra.Command{
	Use:   "tag <path> <tag> [<tag>...]",
	Short: "Add tags to a file or directory",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetPath := args[0]
		newTags := args[1:]

		nodes, err := mapfile.Load(mapfile.DefaultPath)
		if err != nil {
			return err
		}

		node := mapfile.FindByPath(nodes, targetPath)
		if node == nil {
			return fmt.Errorf("path not found in map: %s\nRun 'zesh init' first.", targetPath)
		}

		existing := make(map[string]bool, len(node.Tags))
		for _, t := range node.Tags {
			existing[t] = true
		}
		for _, t := range newTags {
			t = strings.TrimPrefix(t, "#")
			if !existing[t] {
				node.Tags = append(node.Tags, t)
				existing[t] = true
			}
		}

		if err := mapfile.Save(mapfile.DefaultPath, nodes); err != nil {
			return err
		}

		fmt.Printf("tags updated for %s\n", targetPath)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Show metadata for a file or directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetPath := args[0]

		nodes, err := mapfile.Load(mapfile.DefaultPath)
		if err != nil {
			return err
		}

		node := mapfile.FindByPath(nodes, targetPath)
		if node == nil {
			return fmt.Errorf("path not found in map: %s", targetPath)
		}

		output.PrintInfo(node)
		return nil
	},
}

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Print the codebase tree",
	Long: `Prints the file tree from .zesh/objects/map.json.
By default shows language and descriptions. Use --plain for name-only output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, _ := cmd.Flags().GetBool("plain")

		nodes, err := mapfile.Load(mapfile.DefaultPath)
		if err != nil {
			return err
		}

		output.PrintTree(nodes, plain)
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the map in various formats",
	Long:  `Export the codebase map. Use --llm for a flat, token-efficient format suitable for LLM context.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		llm, _ := cmd.Flags().GetBool("llm")
		if !llm {
			return fmt.Errorf("specify an export format, e.g. --llm")
		}

		nodes, err := mapfile.Load(mapfile.DefaultPath)
		if err != nil {
			return err
		}

		output.PrintLLM(nodes)
		return nil
	},
}

func init() {
	initCmd.Flags().Bool("retain", false, "preserve existing descriptions and tags during re-init")
	mapCmd.Flags().Bool("plain", false, "show names only, without descriptions or language")
	exportCmd.Flags().Bool("llm", false, "flat token-efficient format for LLM context")

	rootCmd.AddCommand(initCmd, describeCmd, tagCmd, infoCmd, mapCmd, exportCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
