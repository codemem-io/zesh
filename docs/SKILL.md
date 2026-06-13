# zesh skill

Use `zesh` to maintain and query the codebase map. Run these commands whenever you need to orient yourself in the project, annotate files you've touched, or extract function-level details.

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

zesh get-function --file_name <path> --function_name <name>   # extract function with signature, docs, source, callers
```
