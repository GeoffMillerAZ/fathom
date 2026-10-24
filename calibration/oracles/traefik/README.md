# traefik-oracle (tier A)

The language-breadth harness's oracle for the `traefik` doc-engine dialect (LB-Q-traefik, design §1.3,
§10). A separate Go module (`go.mod`: yaml.v3, go-toml/v2), never imported by fathom, never a product
dependency.

```
cd calibration/oracles/traefik && go build -o <scratch>/traefik-oracle . &&
  <scratch>/traefik-oracle -subject <checkout> -traefik <traefik> \
    -refs-out workspace/tmp/oracle/traefik-<subject>-refs.json \
    -routes-out workspace/tmp/oracle/traefik-<subject>-routes.json \
    -files-out workspace/tmp/oracle/traefik-<subject>-files.txt \
    > workspace/tmp/oracle/traefik-<subject>.json
indexbench lang-oracle -dialect traefik -subject <checkout> -tool-oracle <defs.json> -tool-oracle <refs.json> \
    -only "$(paste -sd, <files.txt>)" -out docs/bench/readings/lang/traefik.json
```

## The instrument

Traefik itself (brew traefik 3.7.13), run with its file provider and its API on a free local port: its own
evaluation of the file, not a reading of it.

- **A dynamic file** (a top-level `http`, `tcp` or `udp`) is the only file of a fresh run (`--providers.file.
  filename=<scratch copy>`, `--api.insecure`, one entry point for the API). `GET /api/rawdata` lists every
  router, service and middleware of the HTTP, TCP and UDP layers as the proxy built them, named
  `<name>@file`. A run that logs "Error while building configuration" (a YAML or TOML error, a template error)
  rejects the file. Traefik renders Go templates before it decodes, so a templated file that is not valid
  YAML or TOML as written is counted `a Go template` and is not a subject.
- **A static file** (a top-level `entryPoints` or `providers`) runs on a scratch copy reduced to what the run
  needs: its entry points moved to free local ports, the API added, `providers.file` the only provider (a
  docker or kubernetes provider needs a daemon) with its filename or directory made absolute against the
  static file's directory. `GET /api/entrypoints` lists its entry points; the dynamic files it loads are the
  ones whose names appear in the same run's rawdata.

Lines are not Traefik output: they come from `gopkg.in/yaml.v3` (YAML) and go-toml's unstable parser (TOML)
for the written file, "where its key is written" (the dialect's rule); a line is a decoder detail, never an
entity decision.

## Tier (design §1.3)

A: the proxy's own evaluation, the reference parser and its template pass, is the truth for what exists. Two
caveats, disclosed: (1) a router can depend on a file the proxy reads at load time (a certificate path) and be
dropped when the file is absent: the oracle runs on the checkout alone, so such a router is a dialect-only
definition (one in the measured subjects, in Heziode/traefik-v2-https-ssl-localhost's `config/dynamic.yml`);
(2) the references are the WRITTEN ones (the proxy's rawdata normalises `auth` to `auth@file`, so the written
form is read from the file and confirmed against the proxy's list of that layer's names).

## What it writes

stdout: definitions (a `package` per accepted file at its first key; a `resource` per entry point; a `route`,
`service` and `rule` per router, service and middleware, the TCP and UDP layers included). `-refs-out`:
references (a router's service and middlewares, a service's weighted, mirroring and failover services, a
middleware chain; a provider suffix `@file` is dropped, and the target must be listed by the proxy),
cross_references (a static file's `providers.file` to the dynamic file the proxy loaded through it, file
granularity; a router's entry point, or its service or middleware declared in exactly one other file the same
static file loaded, entity granularity). `-routes-out` (bench-only): route_references and route_unresolved for
a `loadBalancer.servers` `url` (http) or `address` (tcp, udp), with reasons `external`, `ip-literal`,
`variable`, `unix-socket`, `ambiguous`, `undeclared`, joined to a docker-compose service or Kubernetes Service
of the subject.

`ORACLE_TCC_RESET`: see calibration/oracles/envoy/README.md.

## The route rule and what a route pair is scored at (wave ox, LB-F168 and LB-F169)

`-routes-out` pairs a route's target host with the service that declares it, under one rule: a compose project
is a directory, so a route joins the declarations of its nearest project that declares the host (the route
file's own directory, then each ancestor), one pair for every compose file of that directory that declares the
service (an override file is the same service: compose merges them). Compose files are read by `docker compose config --format json --no-consistency` (Docker Compose 5.1.4: the flag lets an override file's partial service through), so the provider side is tier A; a Kubernetes Service (named as the kubernetes dialect names it, `Service/<namespace>/<name>`, `Service/<name>` with none) and a file Compose refuses are read with yaml.v3, tier C, and make the routes file tier C when a pair joins one. A name no ancestor declares joins only when exactly one file of the subject does, and is `ambiguous` when several do. `lang-routes` scores the routes file at that tier into the xref family.

Where fathom's resolver and this rule disagreed the oracle was the truth and the disagreement was a resolver finding (`.fathom/observations/O435.md`): the resolver fell back from the route's directory to every declaration in the repository. It now walks ancestor directories first (`nearest` in `internal/fusion/routeref.go`).

A subject set under 10 route pairs is too few to read the L3 level honestly (one miss moves xref_precision by
10 points or more): where the corpus joined fewer, public subjects were added, licence recorded in
calibration/corpus.d, and that is the threshold used.

The first eight repositories joined 4 pairs, under the floor; ecamp/ecamp3, picodotdev/blog-ejemplos, rendiffdev/rendiff-dev and wmde/wikibase-suite were added. The 2026-10-24 re-measure on Linux (docker compose config 5.1.4) joins 30 pairs from seven subjects, all found and none beyond the truth, all at tier A (docs/plans/language-breadth.md LB-Q-traefik-rest records the difference from the 2026-10-02 reading's tier C Vistack pair).
