package lsp

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

type serverDef struct {
	binary string
	args   []string
	langID string
}

var serverTable = map[string]serverDef{
	".go":  {binary: "gopls", args: []string{"serve"}, langID: "go"},
	".ts":  {binary: "typescript-language-server", args: []string{"--stdio"}, langID: "typescript"},
	".tsx": {binary: "typescript-language-server", args: []string{"--stdio"}, langID: "typescript"},
	".js":  {binary: "typescript-language-server", args: []string{"--stdio"}, langID: "javascript"},
	".jsx": {binary: "typescript-language-server", args: []string{"--stdio"}, langID: "javascript"},
	".mjs": {binary: "typescript-language-server", args: []string{"--stdio"}, langID: "javascript"},
	".py":  {binary: "pyright-langserver", args: []string{"--stdio"}, langID: "python"},
	".rs":  {binary: "rust-analyzer", args: nil, langID: "rust"},
}

var installHints = map[string]string{
	"gopls":                      "go install golang.org/x/tools/gopls@latest",
	"typescript-language-server": "npm install -g typescript-language-server typescript",
	"pyright-langserver":         "pip install pyright",
	"rust-analyzer":              "rustup component add rust-analyzer",
}

func serverFor(path string) (serverDef, error) {
	ext := filepath.Ext(path)
	def, ok := serverTable[ext]
	if !ok {
		return serverDef{}, fmt.Errorf("no LSP server configured for %q files", ext)
	}
	bin, err := exec.LookPath(def.binary)
	if err != nil {
		hint := installHints[def.binary]
		return serverDef{}, fmt.Errorf("LSP server %q not found in PATH\nInstall with: %s", def.binary, hint)
	}
	def.binary = bin
	return def, nil
}

// Start launches the appropriate LSP server for file and returns a ready Client.
// The returned close func shuts the server down cleanly.
func Start(ctx context.Context, file string) (*Client, func() error, error) {
	def, err := serverFor(file)
	if err != nil {
		return nil, nil, err
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, def.binary, def.args...)
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("lsp stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("lsp stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start %s: %w\nstderr: %s", def.binary, err, stderr.String())
	}

	client := newClient(stdout, stdin)

	closeFn := func() error {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Shutdown(shutCtx)
		_ = client.Exit()
		_ = stdin.Close()
		_ = cmd.Wait()
		return nil
	}

	return client, closeFn, nil
}

// LangIDFor returns the LSP languageId for the given file path.
func LangIDFor(path string) (string, error) {
	def, err := serverFor(path)
	if err != nil {
		return "", err
	}
	return def.langID, nil
}
