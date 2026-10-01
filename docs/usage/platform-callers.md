### `platform-callers`

Print cross-repo callers of a symbol. By default it reads the symbols and call edges
already ingested into the store, so callers are found for **every language** whose
extractor populated them — not just Go.

```sh
fathom platform-callers --store <path> --symbol <name> [--services a,b,c]
```

| Flag | Default | Description |
|---|---|---|
| `--symbol` | (required) | Bare symbol name to find cross-repo callers of |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path, .sqlite/.db (language-agnostic source) |
| `--services` | (all in store) | Comma-separated service IDs to include |
| `--repos` | — | Legacy Go-only mode: comma-separated `id=path` pairs; walks Go source on disk and overrides `--store` when set |

```sh
fathom platform-callers --store workspace/db/fleet.sqlite --symbol ProcessPayment
# cross-repo callers of "ProcessPayment" (store workspace/db/fleet.sqlite (all languages)):
#
# FROM-SERVICE             FROM-FILE                                TO-SERVICE               TO-FILE
# ──────────────────────────────────────────────────────────────────────────────────────────────────
# order-service            internal/checkout/handler.go             payment-service          payment.go
#
# total: 1 caller(s)
```

Cross-repo edges require the index to carry call edges; re-ingest with a current build so
`Package` and `Calls` are populated.

---
