### `platform-ingest`

Ingest all services under a root directory. Each immediate subdirectory is treated as one
service; each gets its own `<service-id>.sqlite` file in the output store directory.

```sh
fathom platform-ingest --root <dir> --store-dir <dir>
```

| Flag | Default | Description |
|---|---|---|
| `--root` | (required) | Directory containing service subdirectories |
| `--store-dir` | (required) | Output directory for per-service `.sqlite` files |

```sh
fathom platform-ingest --root /src/services --store-dir workspace/db/platform
# service:    auth-service
#   discovered: 89
#   extracted:  89
#   symbols:    421
# service:    billing-service
#   ...
# total: 4 services, 421 discovered, 1823 symbols
```

---
