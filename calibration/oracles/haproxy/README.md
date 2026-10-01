# haproxy oracle (LB-Q-haproxy)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the
haproxy native dialect, and the route targets in a file of their own.

| Part | Tier | Tool |
|---|---|---|
| definitions (with end lines), references | A | HAProxy 3.4.6 itself: `haproxy -c -dr -dC0,line -f <file>` parses and checks the file and dumps every statement it parsed with its source line |
| route targets: the compose side | A (C for a version-1 file) | `docker compose config --format json` (Docker Compose 5.1.4); PyYAML for a version-1 file, from `-compose-python`'s interpreter |

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

| `haproxy` | A | `haproxy -c -dr -dC0,line -f <file>` (HAProxy 3.4.6), run per `*.cfg` in its directory; `docker compose config` 5.1.4 | definitions (a `service` per frontend, backend and listen section with its last statement's line as end, a `target` per server, a `rule` per acl line, a `route` per use_backend/default_backend), `references` (a route to a backend of the file); `-routes-out`: each server address resolved against the nearest compose file | `python3 calibration/oracles/haproxy/haproxy_oracle.py -subject <checkout> -haproxy <bin> -out <json> -routes-out <json>` |
