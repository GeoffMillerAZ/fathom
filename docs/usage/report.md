### `report`

Write an analyzer's output as a Markdown finding document — the deterministic bridge
that lets analyzer results flow through the ordinary ingest → compile → claims pipeline
(the `findings` layer) instead of a second persistence path.

```sh
fathom report --store <path> --service <id> [--analyzer security_rollup] [--out findings]
```

Analyzers: `security_rollup` (default; `secret_scan` + `shell_out` + `http_client_calls`
over the service's sources), `authorship` (the Co-Authored-By trailer ledger joined with
staleness and complexity), and `reverse-drift`. Output is exactly
one file, `<service>-<analyzer>.md`, inside `--out`; generation is a pure function of the
corpus working tree, so re-running against an unchanged tree reproduces identical bytes.

#### `report packs` — the dialect and layer pack matrix (OC-31)

```sh
fathom report packs [--json] [--store <path> | --url <daemon>]
```

Every registered language with its extraction engine, typed-edge producer (`native`,
`scip` with its delegate, `syntax`), the fleet's indexed files and projects in it, the
integrity plane's reach (`checked` for Go, `unproven` for other code languages) and the
inventory kinds its files feed; precision and recall only where a published reading
measured them (`console.DialectReadings`, each citing its document and naming the edge
tier it measured — `file` for the TypeScript and Python readings, which is not the
`scip` tier those languages can reach), "unmeasured" — absent, no number — otherwise.
`typed_share` is the share of the store's `typed_edges` rows from the language's files
that a resolver confirmed (a type checker or an imported SCIP index; `file-resolved`
and `unresolved` rows are heuristic), with `edge_kinds` counting them by kind: a
`scip`-level language whose delegate never ran reads a measured 0 ("no SCIP index
imported"), and one with no typed-edge row at all is absent. Layer packs follow with
their source (embedded, project, a kit's `fathom-pack`, `orphan` when no pack defines
a layer the store holds claims for, `unknown` when the pack catalog of a project
holding it could not be read), stratum and the projects where each holds claims. A
project pack that failed to load keeps its load error and has no stratum or role
(never a default), and an unreadable `.fathom/layers` is a note and makes
`layer_count` a lower bound — the union loader's silent embedded-only fallback is not
taken for a complete catalog. `--json` is the console's `/packs` section; the store
resolves as for `layers strata`.

#### `report evidence` — per-service evidence density (CV-39, ADR-044 §5)

```sh
fathom report evidence [--service <id>] [--store DB] [--json]
```

Store-direct, never the daemon. `--store` defaults to `[daemon].store` in
config.toml, else the workspace default (`workspace/db/fathom.sqlite`); the
report names the store it read (`store` in JSON, the first text line). A
store that does not exist, indexes no service, or does not index the named
`--service` is refused (exit 1) naming the store — never an empty file
created on the spot and read back as a row of zeros. `--service` reports
one service; omitted, every service the store's observations table knows
about gets its own row, in `ListServiceIDs`' own alphabetical id order. For
each service:

- **typed share of call edges** — the store's own typed edges
  (`internal/store.TypedEdges`), split by `core.EdgeConfidenceOf` into
  resolved vs. heuristic, as a count and a ratio;
- **languages touched**, each with its edge tier (`typed`/`scip`/
  `file-resolved`/`syntax`, `internal/conserve.TiersByLanguage`) — read from
  the store's own indexed file paths, never a re-read of the corpus, so a
  service whose recorded root no longer exists on this machine still gets
  an honest breakdown;
- **certificate density** — live over declared targets
  (`internal/certify.Gaps`'s own summary), `0` when nothing is declared;
- **inventory item counts by kind** — `internal/analysis/inventory.Extract`
  over the service's own corpus (`api:`/`json:`/`route:`/`env:`/`rpc:`/
  `cli:`/configured-pattern keys), best-effort empty when the corpus is not
  readable from here;
- **whether receipts re-derive** (Go only) — samples the alphabetically
  first non-test Go file's first declaration, builds a
  `symbol_location` receipt against the corpus root's current HEAD, and
  re-derives it from a bare git checkout with `go/parser` alone
  (`internal/receiptverify.Verify`, VB-20): `"live"` when it reproduces,
  `"difference: …"` when it does not, `"unproven: …"`/`"not-applicable:
  …"` when the check could not even be attempted (no corpus root recorded,
  no Go sources, no `go.mod`, not a git checkout) — never defaulted to
  either true or false.

It ranks NO languages: neither the services themselves nor the per-service
language breakdown are ever reordered by how good a language's edge tier
is — both stay in their own alphabetical order regardless of which
service or language happens to have the best tooling support.

---
