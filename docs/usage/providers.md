### `providers`

Auth-health probe across configured LLM profiles — **health, not spend**: each
selected profile's client runs one `Ping` (zero/near-zero tokens). Contrast with `smoke`,
which spends real tokens on a real completion.

```sh
fathom providers [--profile <name>] [--json]
```

Exit `0` when every probeable lane is healthy; exit `1` if any probed lane failed or any
profile could not even be constructed. A profile whose adapter doesn't implement `Ping`
is listed as "not probeable," never silently dropped.

---
