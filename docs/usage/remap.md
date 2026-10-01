### `remap`

Point a service's recorded corpus root at a new location (ADR-017). Because every anchor
in the store is repo-relative, moving a corpus (or the whole store to another machine)
needs exactly this one metadata update — queries, source fetching, freshness watermarks,
and re-ingest all resolve through the remapped path immediately.

```sh
fathom remap --store <path> --service <id> --path <new-corpus-root>
```

`--store`, `--service`, and `--path` are all required; `--path` must be an existing
directory.

---
