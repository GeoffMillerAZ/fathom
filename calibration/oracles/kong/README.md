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

## The route rule and what a route pair is scored at (wave ox, LB-F168 and LB-F169)

`-routes-out` pairs a route's target host with the service that declares it, under one rule: a compose project
is a directory, so a route joins the declarations of its nearest project that declares the host (the route
file's own directory, then each ancestor), one pair for every compose file of that directory that declares the
service (an override file is the same service: compose merges them). Compose files are read by `docker compose config --format json --no-consistency` (Docker Compose 5.1.4: the flag lets an override file's partial service through), so the provider side is tier A; a Kubernetes Service (named as the kubernetes dialect names it, `Service/<namespace>/<name>`, `Service/<name>` with none) and a file Compose refuses are read with yaml.v3, tier C, and make the routes file tier C when a pair joins one. A name no ancestor declares joins only when exactly one file of the subject does, and is `ambiguous` when several do. `lang-routes` scores the routes file at that tier into the xref family.

Where fathom's resolver and this rule disagree the oracle is the truth and the disagreement is a resolver
finding (`.fathom/observations/O435.md`): the resolver falls back from the route's directory to every
declaration in the repository, so a service named in one sandbox's route joins the same-named service of every
other sandbox.

A subject set under 10 route pairs is too few to read the L3 level honestly (one miss moves xref_precision by
10 points or more): where the corpus joined fewer, public subjects were added, licence recorded in
calibration/corpus.d, and that is the threshold used.

A route pair is tier A where decK attests both ends: the file is one decK accepts, decK's rendered state holds the
written host for the same service or upstream (`renderedHosts`; a host it does not hold is counted on stderr and
makes the pair tier C), and Docker Compose read the compose side. Pairs from a file decK refuses (the DB-less
loader's acls, credentials and comments) are written text and tier C. `-routes-out` carries the weakest tier of
its pairs (`TestRoutesFileTier`, `TestRenderedHosts`), which `lang-routes` scores into the xref family. The
reading's 12 pairs (callistaenterprise/blog-api-gateway-kong 4, afteracademy/gomicro 6,
jonashackt/spring-boot-openapi-kong 1, AliBigdeli/FastApi-GRPC-Todo-Microservice-App 1) come from decK-accepted
files of the seven compose-paired repositories and are tier A; supabase/supabase's kong.yml is the DB-less
loader's, decK refuses it, no definition of it is attested (lang-oracle would count its dialect definitions
against an empty oracle) and its 14 tier-C pairs are not scored. decK v1.68.0 darwin_all release: tarball sha256
`78e1cc4b9257483e84ca7bc391cec9a0a0c47bffc47ed2f1266263205e4a585b`, binary sha256
`fa560e40045a63987968d18807af38ffe26476aa38b1fd8a6d7d1e1e479e8cc2`.
