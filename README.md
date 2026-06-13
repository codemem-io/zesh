# zesh

A codebase map tool for humans and LLMs. Scan your project once, annotate files with descriptions and tags, and export a structured map that makes any codebase instantly navigable — by you or by an AI agent.

---

## Install

```bash
git clone https://github.com/codemem-io/zesh
cd zesh
go build -o zesh ./cmd/zesh
```

**LSP features** (`get-function`) require the language server for your language:

| Language | Install |
|---|---|
| Go | `go install golang.org/x/tools/gopls@latest` |
| TypeScript / JS | `npm install -g typescript-language-server typescript` |
| Python | `pip install pyright` |
| Rust | `rustup component add rust-analyzer` |

---

## Commands

### `zesh init`

Scan the current directory and write `.zesh/objects/map.json`.

```bash
zesh init              # fresh scan
zesh init --retain     # preserve existing descriptions and tags
```

Respects `.zeshignore` (or `.gitignore` if no `.zeshignore` exists).

---

### `zesh map`

Print the file tree from the map.

```bash
zesh map               # tree with language and descriptions
zesh map --plain       # names only
```

---

### `zesh describe`

Annotate a file or directory.

```bash
zesh describe internal/auth/auth.go "JWT middleware and token validation"
```

---

### `zesh tag`

Add tags to a file or directory.

```bash
zesh tag internal/auth/auth.go auth security middleware
```

---

### `zesh info`

Inspect metadata for a single path.

```bash
zesh info internal/auth/auth.go
```

---

### `zesh export`

Export the map for LLM consumption.

```bash
zesh export --llm      # flat token-efficient format
```

---

### `zesh get-function`

Extract a named function with LSP-enriched metadata: signature, doc comment, full source, and call sites.

```bash
zesh get-function --file_name internal/auth/auth.go --function_name ValidateToken
```

Requires the language server for the file's language to be installed.

---

## How it works

```
zesh init
  └─ walks directory tree
  └─ detects language per file extension
  └─ writes .zesh/objects/map.json

zesh describe / tag
  └─ loads map.json
  └─ updates the node in place
  └─ saves back to disk

zesh export --llm
  └─ serialises map.json into a compact text format
  └─ paste into your LLM context or pipe to a file

zesh get-function
  └─ starts the language server (gopls, tsserver, pyright, rust-analyzer)
  └─ opens the file via LSP
  └─ queries documentSymbol → hover → references
  └─ prints signature, docs, source, and callers
```

---

## The map file

`.zesh/objects/map.json` is plain JSON. You can read it, version it, and pipe it anywhere.

```json
[
  {
    "name": "main.go",
    "path": "cmd/zesh/main.go",
    "type": "file",
    "language": "go",
    "description": "CLI entry point and command registration",
    "tags": ["entrypoint"],
    "size_bytes": 4200,
    "last_modified": "2026-06-13T00:00:00Z"
  }
]
```

---

## Ignoring files

Create `.zeshignore` in the project root using the same syntax as `.gitignore`. If no `.zeshignore` exists, zesh falls back to `.gitignore`. `.git` is always excluded.

---

## Claude Code skill

See [SKILL.md](docs/SKILL.md) for a ready-to-use reference that lets Claude Code agents use zesh commands effectively when working in your codebase.
