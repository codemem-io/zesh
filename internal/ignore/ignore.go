package ignore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load reads ignore patterns from .zeshignore or .gitignore in root.
// .git is always appended.
func Load(root string) ([]string, error) {
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
	patterns = append(patterns, ".git")
	return patterns, nil
}

// Match reports whether name matches any of the patterns.
func Match(name string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}
