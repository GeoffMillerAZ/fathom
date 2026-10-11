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

### The dry-run estimate and the daemon path

The dry run prices output from the service's own history (EW-74): the out/in token
ratio range of its completed, non-dry `compile_runs` (the same layer's when it has
any, else its other layers'), else the default band x2 to x8. The `tokens_out` line
names the ratio and where it came from; `cost_usd` is a range, or `unpriced` for a
model this binary has no price for, never a zero. The `--store` and daemon paths print
the same lines (the daemon's `compile_layer` payload carries `est_tokens_out_low/high`,
`est_cost_low/high_usd`, `est_out_ratio_low/high`, `est_out_basis`, `price_known` and,
for sizing a spend, `est_section_ms`).

A spending compile through the daemon (EW-73) first asks the daemon for that estimate,
then waits sections-to-compile x the service's past wall time per section (40 s with no
history) x 1.5, never under 120 s nor over 2 h, and prints the wait on stderr. The
daemon lifts its own write deadline for `compile_layer`; the client timeout is the only
bound left.

Both guard-override flags are logged into the resulting `compile_runs` row, never applied
silently. See [docs/layers.md](../layers.md) and [docs/configuration.md](../configuration.md)
for the provider-lane and layer-catalog references.

---
