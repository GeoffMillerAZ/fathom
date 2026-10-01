# nginx oracle (LB-Q-nginx)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the
nginx native dialect, and the route targets in a file of their own.

| Part | Tier | Tool |
|---|---|---|
| definitions, references, cross_references (includes) | C | nginxinc crossplane 0.5.8 (Apache-2.0), NGINX's Python parser: an independent parser library (design §1.3) |
| included file set (cross-check) | A | `nginx -T` (nginx 1.31.6, built from nginx.org source into scratch) on a scratch copy of each nginx.conf's prefix directory |
| route targets: the compose side | A (C for a version-1 file) | `docker compose config --format json` (Docker Compose 5.1.4); PyYAML 6.0.3 for a version-1 file Compose 5 refuses |

## Install (scratch, on the external volume)

```sh
python3 -m venv <scratch>/venv && <scratch>/venv/bin/pip install -r calibration/oracles/nginx/requirements.txt
# nginx 1.31.6 (optional, the tier-A cross-check): from https://nginx.org/download/nginx-1.31.6.tar.gz,
# ./configure --prefix=<scratch>/nginx --with-pcre --with-http_ssl_module --with-stream ... && make install
```

## Run

```sh
<scratch>/venv/bin/python calibration/oracles/nginx/nginx_oracle.py -subject <checkout> \
    -out workspace/tmp/oracle/nginx-<subject>.json -routes-out workspace/tmp/oracle/nginx-<subject>-routes.json \
    [-nginx <scratch>/nginx/sbin/nginx -scratch <scratch dir>] [-summary-out <file>]
go run ./cmd/indexbench lang-oracle -dialect nginx -subject <checkout> \
    -tool-oracle workspace/tmp/oracle/nginx-<subject>.json -out docs/bench/readings/lang/nginx.json
```

`-routes-out` holds `route_references` (`path`, `from`, `to_path`, `to`, `raw`) and
`route_unresolved` (`path`, `from`, `raw`, `reason`: variable, unix-socket, ip-literal, ambiguous,
external, undeclared). lang-oracle's tool-oracle decoder refuses unknown fields, so these are not in
the main file; LB-F98's route measurement reads them.

## What the truth is

The module docstring (`nginx_oracle.py`) states each rule: the naming contract is the dialect's
(`internal/extractor/dialects/nginx/PROVENANCE.md`); includes resolve against the nearest nginx.conf
at or above the including file (nginx's configuration prefix), `/etc/nginx/` mapped onto it, globs
expanded as glob(3) does. `nginx -T` cannot load a configuration that names a host this machine
cannot resolve (a compose service: "host not found in upstream") or a dynamic module or a variable
only an image defines; the summary names each failure. On the measured subjects it passed on
h5bp/server-configs-nginx (8 of 8 files agree with the oracle's include closure), nginx/nginx's conf/
(2 of 2) and awesome-compose's react-nginx; it failed on the compose samples (host not found) as
expected.

## Index row (calibration/oracles/README.md)

| `nginx` | C (definitions, references, cross_references); A (`nginx -T` cross-check, the compose side of routes) | crossplane 0.5.8 `parse(single=True)` per file; `nginx -T` 1.31.6 on a scratch copy of each prefix; `docker compose config` 5.1.4 | definitions (a `package` per file, a `service` per server and upstream, a `route` per location, a `target` per upstream server), `references` (a pass to an upstream of the file), `cross_references` (an include, prefix-relative, file granularity); `-routes-out`: route_references/route_unresolved | `<venv>/bin/python calibration/oracles/nginx/nginx_oracle.py -subject <checkout> -out <json> -routes-out <json> [-nginx <bin> -scratch <dir>]` |
