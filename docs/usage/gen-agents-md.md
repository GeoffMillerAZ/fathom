### `gen-agents-md`

Generate an `AGENTS.md` knowledge index from stored symbols. The output is a Markdown file
suitable for dropping into an AI coding assistant's context (ADR-059: AGENTS.md is the one
instruction file). `gen-claude-md` is its deprecated alias through 0.9.x: it keeps its old
default output (`workspace/artifacts/CLAUDE.md`) and prints a deprecation line on stderr.

```sh
fathom gen-agents-md --service <id> [--store <path>] [--output <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
| `--output` | `workspace/artifacts/AGENTS.md` | Output file path |

Output is deterministic: symbols are sorted by kind then name. The file includes a
kind/count breakdown table and a full symbol listing.

```sh
fathom gen-agents-md --service my-service
# wrote workspace/artifacts/AGENTS.md (891 symbols)
```

---
