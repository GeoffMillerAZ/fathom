# caddyfile oracle (LB-Q-caddyfile)

Bench-only; never a product dependency (its own Go module, never imported by fathom). Writes
`indexbench lang-oracle`'s tool-oracle JSON for the caddyfile native dialect, and the route targets
in a file of their own.

| Part | Tier | Tool |
|---|---|---|
| which files count, the upstreams dialed | A | `caddy adapt --config <file> --adapter caddyfile` (Caddy v2.11.6): a file it rejects is left out; its JSON's `dial` addresses cross-check the upstreams |
| tokens, lines, quoting | A | `caddyfile.Tokenize`, the reference lexer of github.com/caddyserver/caddy/v2 v2.11.6, as a library |
| site blocks (cross-check) | A | `caddyfile.Parse`, the reference parser: its server blocks' first keys against the sites read |
| route targets: the compose side | A | `docker compose config --format json --no-consistency` (Docker Compose 5.1.4) on every compose file of the nearest project directory |

What is the oracle's own: grouping the reference tokens into statements and blocks (a token on a
later line than the previous token's end starts a statement; a trailing unquoted `{` opens a block,
a lone `}` closes it) and the naming contract, which is the dialect's
(`internal/extractor/dialects/caddyfile/PROVENANCE.md`). Caddy itself defines no "nested directive"
(each directive's unmarshaler reads its block), so the entity rules are the contract applied to the
reference tokens; what Caddy attests is the token stream, the file's validity, the site keys and the
upstreams it dials (stderr and `-summary-out` count agreement).

A subject built with plugins is adapted by a caddy built the way its Dockerfile builds one, and
`-env` supplies its deployment's `{$NAME}` placeholders (Caddy substitutes them before parsing; an
empty one can leave a site with no address). picosh/pico: `xcaddy build --with
github.com/caddy-dns/cloudflare --with github.com/yroc92/postgres-storage`, reproduced as a
`caddycmd.Main()` program at caddy v2.11.6, cloudflare v0.2.4, postgres-storage
v0.0.0-20230310195636-276797aefe40, with `-env APP_DOMAIN=pico.test -env APP_EMAIL=ops@pico.test
-env CF_API_TOKEN=token -env DATABASE_URL=postgres://db/pico -env
MONITORING_APP_DOMAIN=monitor.pico.test -env MONITORING_APP_EMAIL=ops@pico.test`.

## Run

```sh
cd calibration/oracles/caddyfile && go build -o <scratch>/caddyfile-oracle . && cd -
# caddy's caddyfile_adapt tests: materialize each .caddyfiletest's Caddyfile half into a scratch subject
<scratch>/caddyfile-oracle -subject <scratch>/caddy-adapt-subject \
    -materialize-from workspace/calibration/caddyfile/caddy/caddytest/integration/caddyfile_adapt \
    -caddy <caddy> -out workspace/tmp/oracle/caddy-adapt.json -files-out <files> -routes-out <json>
go run ./cmd/indexbench lang-oracle -dialect caddyfile -subject <scratch>/caddy-adapt-subject \
    -subject-name caddyfile_adapt -tool-oracle workspace/tmp/oracle/caddy-adapt.json \
    -only "$(paste -sd, <files>)" -out docs/bench/readings/lang/caddyfile.json
```

Of the 244 caddyfile_adapt tests, `caddy adapt` rejects 48 (tests of an error, a module the stock
build does not carry): `-files-out` lists the 196 it accepts and `-only` restricts the measurement to
them.

## Index row (calibration/oracles/README.md)

| `caddyfile` | A | `caddy adapt` v2.11.6 per file (acceptance, dials), `caddyfile.Tokenize`/`caddyfile.Parse` of caddy v2.11.6 as a library; `docker compose config` 5.1.4 | definitions (a `package` per file, a `service` per site, a `mixin` per snippet, a `route` per named route, route block and proxy directive, a `selector` per named matcher, each with its end line), `references` (an import of a snippet, an invoke of a named route), `cross_references` (a file import); `-routes-out`: each upstream resolved against the nearest compose project, a pair per declaring compose file | `go build -o <bin> . && <bin> -subject <dir> -caddy <caddy> -out <json> -files-out <file> -routes-out <json> [-materialize-from <dir>] [-env K=V]...` |

## The route rule and what a route pair is scored at (wave ox, LB-F168 and LB-F169)

`-routes-out` pairs a route's target host with the service that declares it, under one rule: a compose project
is a directory, so a route joins the declarations of its nearest project that declares the host (the route
file's own directory, then each ancestor), one pair for every compose file of that directory that declares the
service (an override file is the same service: compose merges them). Compose files are read by `docker compose config --format json --no-consistency` (Docker Compose 5.1.4: the flag lets an override file's partial service through), tier A; a version-1 file Compose refuses is read with PyYAML, tier C. A name no ancestor declares is `undeclared` (`external` when dotted), several services behind one name `ambiguous`.

Where fathom's resolver and this rule disagree the oracle is the truth and the disagreement is a resolver
finding (`.fathom/observations/O435.md`): the resolver falls back from the route's directory to every
declaration in the repository, so a service named in one sandbox's route joins the same-named service of every
other sandbox.

A subject set under 10 route pairs is too few to read the L3 level honestly (one miss moves xref_precision by
10 points or more): where the corpus joined fewer, public subjects were added, licence recorded in
calibration/corpus.d, and that is the threshold used.

picosh/pico's Caddyfiles sit under `caddy/` and its root holds docker-compose.yml with three overlays (override,
prod, prod-irc) that declare the same services: 20 pairs (6 distinct services, each to every file that declares
it), `lang-routes` agreement 20 of 20 at tier A, above the floor with no subject added. `go test` in this module
holds the rule (`TestComposeProjectRule`). Tools: caddy v2.11.6 stock (sha256
`1cf492f41b9bc2465dfef450eeafdff78789c5af9da9b3d9614320f8a9fde526`) and, for pico, the plugin build (cloudflare
v0.2.4, postgres-storage v0.0.0-20230310195636-276797aefe40; sha256
`b85402b58b74ab90c40496853158e314c6d1e4caa4340516ff2557e129d6f2bd`).
