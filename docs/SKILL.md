# zesh skill

Use `zesh` to maintain and query the codebase map. Run these commands whenever you need to orient yourself in the project, annotate files you've touched, or extract symbol-level details.

## Commands

```bash
zesh init                                          # scan directory, write .zesh/objects/map.json
zesh init --retain                                 # re-scan, keep existing descriptions and tags

zesh map                                           # print file tree with language and descriptions
zesh map --plain                                   # names only

zesh describe <path> "<description>"               # set description for a file or directory
zesh tag <path> <tag> [<tag>...]                   # add tags to a file or directory
zesh info <path>                                   # show metadata for a file or directory

zesh export --llm                                  # export map in flat LLM-friendly format

zesh find <keyword>                                # search symbols project-wide by name
zesh find <keyword> --path <file-or-dir>           # scope search to a file or directory

zesh inspect --file <path> --symbol <name>         # extract full details for a specific symbol
zesh get-function --file_name <path> --function_name <name>   # extract function (signature, docs, source, callers)
```

## Symbol discovery workflow

When you need to understand a function, method, or type — and don't know exactly where it lives:

```bash
# 1. Search by keyword (case-insensitive substring match)
zesh find Hover

# Output:
# NAME              KIND    FILE:LINE
# Client.Hover      method  internal/lsp/client.go:181
# HoverParams       struct  internal/lsp/protocol.go:82
# Hover             struct  internal/lsp/protocol.go:87

# 2. Use the exact NAME from the output to inspect it
zesh inspect --file internal/lsp/client.go --symbol "Client.Hover"
```

`find` returns the exact symbol name. Pass it verbatim to `inspect` via `--symbol`.
