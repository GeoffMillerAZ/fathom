### `compile`

Compile a semantic layer for a service — an LLM pass over the layer's corpus that emits
governed claims (ADR-016). `dry_run` defaults to `true`: it estimates cost without
calling the LLM or writing anything, printing a real `cost_usd` range for the model
`--profile`/`FATHOM_COMPILE_MODEL` resolves when this binary knows a price for it, or
`cost unknown: no price for model <m>` when it does not — never a fabricated or
zero-shaped estimate.

```sh
fathom compile --service <id> [--layer obligations] [--dry-run=false] \
  [--store <path> [--profile <name>] [--force] [--unsafe-skip-eval-gate]] \
  [--url http://localhost:7575]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--layer` | `""` (the default compile set) | Layer to compile (must be defined in the manifest; `fathom layers list` shows what's visible). Empty compiles CV-41's default set — `semantic.DefaultLayersFor`: `obligations` always, plus `adr-rules`/`spec-bindings` when their own corpus matches at least one file under the service's root — one layer at a time, direct `--store` mode only. HTTP mode has no plumbing to resolve the service's root from the CLI, so an empty `--layer` there still compiles `obligations` alone, unchanged from before this row. |
| `--dry-run` | `true` | Estimate cost without spending; `--dry-run=false` to actually compile |
| `--store` | `""` | Direct mode — bypasses the daemon, compiles against a local store |
| `--url` | `defaultFathomURL()` | HTTP mode base URL (used when `--store` is empty; the daemon compiles with its own configured profile) |
| `--profile` | `""` | Named LLM profile from `~/.fathom/config.toml` (direct `--store` mode only) |
| `--force` | `false` | Override the idempotency guard (same service/layer/watermark/prompt already compiled) — direct `--store` mode only |
| `--unsafe-skip-eval-gate` | `false` | Override the refusal to compile a layer with no passing eval record — direct `--store` mode only |

Both guard-override flags are logged into the resulting `compile_runs` row, never applied
silently. See [docs/layers.md](../layers.md) and [docs/configuration.md](../configuration.md)
for the provider-lane and layer-catalog references.

---
