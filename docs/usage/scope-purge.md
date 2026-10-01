### `scope` / `purge` — reversible suppression and verified removal

```sh
fathom scope list      [--all] [--service S] [--json]
fathom scope suppress  '<glob>' --reason R [--service S] [--until 7d]
fathom scope unsuppress <id>
fathom scope preview   '<glob>' [--service S] [--store DB] [--sample 10] [--json]
fathom purge '<glob>' --service S --reason '<why>' [--store DB] [--dry-run | --yes] \
     [--all] [--probe <literal>]... [--include-history --force] [--json]
fathom purge history [--json]
```

`scope suppress` hides matching paths from query results (reversible,
reason-carrying, disclosed in every MCP envelope's `suppressed` block);
`purge` removes matching content from the store irreversibly, appends a
service-scoped ignore so re-ingest cannot resurrect it, and its verdict is
tri-state with a post-purge byte probe — exit `2` means the SQL rows are
gone but the raw store bytes were left unchecked. `--all` is the glob
`**`; `--include-history` also deletes `claim_events` (breaks claim-history
integrity) and needs `--force` as a second key. See
`docs/use-cases/fleet-health.md` and `docs/design/scope-and-purge.md`.
