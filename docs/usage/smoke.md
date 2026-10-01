### `smoke`

One tiny real completion per selected profile — reports model, latency, tokens, and cost.
Because this spends real money, `--profile` or `--all` is **required**; there is no
implicit "smoke everything" default.

```sh
fathom smoke --profile <name> [--json]
fathom smoke --all [--json]
```

Cost is estimated at the same flat per-KTok rates `compile` uses, so a smoke run's
reported cost is directly comparable to a compile run's `cost_usd`.

---
