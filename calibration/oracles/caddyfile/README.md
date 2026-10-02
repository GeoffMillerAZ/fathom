# caddyfile oracle (LB-Q-caddyfile)

Bench-only; never a product dependency (its own Go module, never imported by fathom). Writes
`indexbench lang-oracle`'s tool-oracle JSON for the caddyfile native dialect, and the route targets
in a file of their own.

| Part | Tier | Tool |
|---|---|---|
| which files count, the upstreams dialed | A | `caddy adapt --config <file> --adapter caddyfile` (Caddy v2.11.6): a file it rejects is left out; its JSON's `dial` addresses cross-check the upstreams |
| tokens, lines, quoting | A | `caddyfile.Tokenize`, the reference lexer of github.com/caddyserver/caddy/v2 v2.11.6, as a library |
| site blocks (cross-check) | A | `caddyfile.Parse`, the reference parser: its server blocks' first keys against the sites read |
| route targets: the compose side | A | `docker compose config --format json` (Docker Compose 5.1.4) |

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

| `caddyfile` | A | `caddy adapt` v2.11.6 per file (acceptance, dials), `caddyfile.Tokenize`/`caddyfile.Parse` of caddy v2.11.6 as a library; `docker compose config` 5.1.4 | definitions (a `package` per file, a `service` per site, a `mixin` per snippet, a `route` per named route, route block and proxy directive, a `selector` per named matcher, each with its end line), `references` (an import of a snippet, an invoke of a named route), `cross_references` (a file import); `-routes-out`: each upstream resolved against the nearest compose file | `go build -o <bin> . && <bin> -subject <dir> -caddy <caddy> -out <json> -files-out <file> -routes-out <json> [-materialize-from <dir>] [-env K=V]...` |
