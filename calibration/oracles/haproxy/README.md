# haproxy oracle (LB-Q-haproxy)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the
haproxy native dialect, and the route targets in a file of their own.

| Part | Tier | Tool |
|---|---|---|
| definitions (with end lines), references | A | HAProxy 3.4.6 itself: `haproxy -c -dr -dC0,line -f <file>` parses and checks the file and dumps every statement it parsed with its source line |
| route targets: the compose side | A (C for a version-1 file) | `docker compose config --format json --no-consistency` (Docker Compose 5.1.4) on every compose file of the nearest project directory; PyYAML for a version-1 file, from `-compose-python`'s interpreter |

`-dr` keeps a server address this machine cannot resolve (a compose service name) from failing the
check; a file HAProxy rejects (exit 1) is counted and left out, so a `.cfg` of another format is
never a definition. HAProxy reads a file twice (a first pass for `global`), so the dump holds it twice
and the first pass is kept.

## Install (scratch, on the external volume)

HAProxy 3.4.6 from https://www.haproxy.org/download/3.4/src/haproxy-3.4.6.tar.gz (sha256
`791e1815f8af6e8b850a227a9a0a190f3d3478c9e8d38a0f51c98b7f4bfe368b`):
`make TARGET=osx USE_OPENSSL=1 USE_PCRE2=1 USE_ZLIB=1 SSL_INC=... SSL_LIB=... PCRE2_INC=... PCRE2_LIB=...`,
the binary copied to `<scratch>/haproxy/haproxy`. The script needs only Python 3's standard library
(PyYAML only for a version-1 compose file, through `-compose-python`).

## Run

```sh
python3 calibration/oracles/haproxy/haproxy_oracle.py -subject <checkout> -haproxy <scratch>/haproxy/haproxy \
    -out workspace/tmp/oracle/haproxy-<subject>.json -routes-out workspace/tmp/oracle/haproxy-<subject>-routes.json \
    [-compose-python <venv>/bin/python] [-summary-out <file>]
go run ./cmd/indexbench lang-oracle -dialect haproxy -subject <checkout> \
    -tool-oracle workspace/tmp/oracle/haproxy-<subject>.json -out docs/bench/readings/lang/haproxy.json
```

The route file's shape is the nginx oracle's (`route_references`, `route_unresolved` with a reason).

## Index row (calibration/oracles/README.md)

| `haproxy` | A | `haproxy -c -dr -dC0,line -f <file>` (HAProxy 3.4.6), run per `*.cfg` in its directory; `docker compose config` 5.1.4 | definitions (a `service` per frontend, backend and listen section with its last statement's line as end, a `target` per server, a `rule` per acl line, a `route` per use_backend/default_backend), `references` (a route to a backend of the file); `-routes-out`: each server address resolved against the nearest compose project, a pair per declaring compose file | `python3 calibration/oracles/haproxy/haproxy_oracle.py -subject <checkout> -haproxy <bin> -out <json> -routes-out <json>` |

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

Subjects: haproxy/haproxy examples (no compose file) and bingohuang/docker-labs (4 pairs), then for the floor
eclipse-steady/steady (9 pairs), adminsuaib/NexSaleAi (3) and SpaaceCadet/Docker-compose-HAProxy-Keepalived (3):
19 pairs, `lang-routes` agreement 19 of 19 at tier A. `python3 -m unittest test_haproxy_oracle` (in this
directory) holds the rule's tests. Tools of the wave-ox run: HAProxy 3.4.6-56332c5 (binary sha256
`479efcf45167ad5f94e464a5d274183d2cd5ca7504f6802e0f35f6a123207792`), Docker Compose 5.1.4.
