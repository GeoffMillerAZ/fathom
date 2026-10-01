### `config`

Print the effective configuration: user + project `.fathom/config.toml` layers merged,
the active profile resolved (flag `>` env `>` project file `>` user file `>` built-in
default), printed as human text or JSON.

```sh
fathom config [--profile <name>] [--explain] [--json]
```

`--explain` is the flagship flag — it annotates every resolved key with its provenance
(which layer supplied it, and the exact source file or env var). Pure local file reads,
no daemon dependency.

---
