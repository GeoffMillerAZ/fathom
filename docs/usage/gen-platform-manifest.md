### `gen-platform-manifest`

Generate a platform manifest YAML from `*.sqlite`/`*.db` store files (a stray `*.duckdb`
file is still listed for visibility if it has no `.sqlite` sibling, but opening it will
fail loudly — DuckDB stores are no longer supported, SS-8). The manifest lists service
IDs and their store paths, suitable for feeding to `serve-http` platform mode.

```sh
fathom gen-platform-manifest --store-dir <dir> [--output <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--store-dir` | (required) | Directory containing `*.sqlite`/`*.db` files |
| `--output` | `workspace/artifacts/platform-manifest.yaml` | Output YAML path |

```sh
fathom gen-platform-manifest --store-dir workspace/db/platform
# wrote workspace/artifacts/platform-manifest.yaml (4 services)
```

---
