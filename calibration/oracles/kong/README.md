# kong-oracle (tier A definitions; tier C references)

The language-breadth harness's oracle for the `kong` doc-engine dialect (LB-Q-kong, design §1.3, §10). A
separate Go module (`go.mod`: yaml.v3), never imported by fathom, never a product dependency.

```
cd calibration/oracles/kong && go build -o <scratch>/kong-oracle . &&
  <scratch>/kong-oracle -subject <checkout> -deck <deck> -scratch <dir outside the subject> \
    -refs-out workspace/tmp/oracle/kong-<subject>-refs.json \
    -routes-out workspace/tmp/oracle/kong-<subject>-routes.json \
    -files-out workspace/tmp/oracle/kong-<subject>-files.txt \
    > workspace/tmp/oracle/kong-<subject>.json
indexbench lang-oracle -dialect kong -subject <checkout> -tool-oracle <defs.json> -tool-oracle <refs.json> \
    -only "$(paste -sd, <files.txt>)" -out docs/bench/readings/lang/kong.json
```

## The instrument

decK (Kong's own declarative-configuration tool, Apache-2.0), the v1.68.0 release binary (the brew-core
`deck` 1.24.2 is a different, MIT tool), offline, on every file that declares `_format_version`:

- `deck file validate <file>`: the acceptance. A file decK refuses (its schema is stricter than the DB-less
  loader's: no `acls`, credentials or `_comment` at the root, no top-level `targets`, `route.service` an object)
  contributes no entity and is counted with its first error. The files it refuses are not hidden: their
  service hosts and upstream targets still feed `route_references`, marked tier C.
- `deck file render --format json <file>`: the entity truth, decK's normalised state with defaults filled in
  and every foreign key resolved into nesting (a route under the service it names; a plugin under the service
  or route it is scoped to). An entity the state holds is a definition; one it does not (an unnamed route) is not.

Each rendered entity is paired with the written one of the same kind, name and effective scope through
`gopkg.in/yaml.v3` for its line (decK prints no source lines); a rendered entity no written one pairs with
drops the file, counted as unpairable.

## Tiers (design §1.3)

Definitions (stdout): tier A. decK is the format's own tool, and the entities are what its state holds.
References (`-refs-out`): tier C. decK's rendering says a route sits under a service; it does not say whether
the route WROTE `service:` or was written under it, and the dialect, which reads downward and has no parent
context, can only see the written one. The oracle's references are the written foreign keys (a route's
`service`, a plugin's `service` and `route`, a service's `host` naming an upstream of the file), each
cross-checked against decK's own nesting or its rendered upstream. A tier-C L2 caps at good; the ceiling is
LB-Q-kong-rest.

## What it writes

cross_references (tier C): a reference whose name is declared in no entity of its own file and in exactly one
entity of another file of the same directory (the files decK merges), entity granularity. route_references
and route_unresolved (`-routes-out`, bench-only): a service `host` or `url` that is not an upstream of the
file, and an upstream target, joined to a docker-compose service or Kubernetes Service of the subject, or the
reason (`external`, `ip-literal`, `variable`, `unix-socket`, `ambiguous`, `undeclared`). `lang-oracle` refuses
a tool oracle with a field it does not know, hence the separate file.

## Subjects

Kong/deck and Kong/go-database-reconciler (Apache-2.0) are the only subjects that write foreign keys in bulk,
and they keep every state file under `testdata/`, which Discover prunes (`internal/connector.skipDir`): ingest
never reads them. They are measured from a scratch copy with the path component `testdata` renamed `td`
(`cp` each yaml with the rename; the reading's notes say so). The real-repository subjects (a `kong.yml`
beside a docker-compose stack) write routes nested under services, so they measure L1 and, through
`-routes-out`, the route layer.

`ORACLE_TCC_RESET`: see calibration/oracles/envoy/README.md.
