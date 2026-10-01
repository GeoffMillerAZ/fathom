### `query`

Query symbols for a concern. ConcernID is mandatory (ADR-006).

```sh
fathom query --service <id> --concern <concern-id> [--keywords <kw,...>] [--top <n>] [--store <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--concern` | (required) | Concern ID to scope the query, e.g. `authentication`, `billing` |
| `--keywords` | `""` | Comma-separated keywords for relevance scoring |
| `--top` | `20` | Maximum results to return |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |

When the concern-scoped query returns no results, the command falls back to a keyword
substring search over all symbols for the service.

```sh
fathom query --service payment-service --concern auth --keywords token,validate --top 5
#   1  ValidateToken                             GoFunc  internal/auth/token.go
#      func ValidateToken(ctx context.Context, tok string) error
#   2  RefreshToken                              GoFunc  internal/auth/token.go
#   ...
```

If staleness is detected, a warning is printed at the end of the output:
`warning: staleness detected — re-ingest recommended`

---
