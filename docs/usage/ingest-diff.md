### `ingest-diff`

Ingest only files changed since a git ref. When `--base` is omitted, falls back to a full
ingest.

```sh
fathom ingest-diff --service <id> [--path <dir>] [--base <git-ref>] [--store <path>] [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID |
| `--path` | `.` | Source root directory |
| `--base` | `""` | Git ref to diff against (e.g. `HEAD~1`, `main`). Empty = full ingest |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
| `--mode` | `local` | Connector mode |

```sh
fathom ingest-diff --service payment-service --path /src/payment --base main
# service:    payment-service
# base:       main
# changed:    7 files
# discovered: 142
# ...
```

---
