# nginx oracle (LB-Q-nginx)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the
nginx native dialect, and the route targets in a file of their own.

| Part | Tier | Tool |
|---|---|---|
| definitions, references, cross_references (includes) | B (C without `-nginx`) | nginxinc crossplane 0.5.8 (Apache-2.0), NGINX Inc's parser tool, cross-checked against nginx itself (below) |
| scope and cross-check | A (nginx's verdict on a file) | `nginx -T` (nginx 1.31.6, `brew install nginx`) on a scratch copy of each file's prefix directory: a file is in scope only where nginx loads it |
| route targets: the compose side | A (C for a version-1 file) | `docker compose config --format json --no-consistency` (Docker Compose 5.1.4) on every compose file of the nearest project directory; PyYAML 6.0.3 for a version-1 file Compose 5 refuses |

## Why crossplane is tier B, not C (LB-F170)

The relabel from C to B, with its reason. On the ladder in calibration/oracles/envoy/README.md (the format's
reference parser is A, a widely used third-party parser tool is B, an independent library written against the
format is C), crossplane is NGINX Inc's own parser tool for the format and emits nginx's directive tree with its
lines. Design §1.5 D2 also places an independent parser's syntax dump at B (php-parser, luaparse), where §1.3's
table lists "an independent parser library" under C: this oracle is the tool and the dump, so D2's B is the
definition it meets. It is not A because crossplane reads text; nginx's configuration loader is the reference.
The relabel is earned by a cross-check against nginx itself, not claimed: `nginx -T` on a scratch copy of each
prefix must load each file in scope, and crossplane must parse the same directives from nginx's own dump of the
file; a file nginx does not load, or whose dump parses differently, is excluded and counted (stderr and
`-summary-out`). Tier B proves L1 excellent (sym_precision >= .98) and L2 and L3 good (`langbench.tierCeiling`).

Measured 2026-10-02 (nginx 1.31.6, crossplane 0.5.8): nginx loaded 47 of the 86 files crossplane parses in the
four subjects (h5bp/server-configs-nginx 33 of 38, nginx/nginx conf 2 of 2, docker/awesome-compose 1 of 8,
nginxinc/NGINX-Demos 11 of 38); the rest name a compose service nginx cannot resolve here, a certificate, a module
or an NGINX Plus directive the repository or this build lacks, and are counted out. No dump disagreed with its
file. L1 reads sym_precision 1 and sym_recall 1 (71 definitions) at tier B: excellent.

The route pairs (a `proxy_pass` host joined to a compose service) are not scored: the reading is taken with
`lang-oracle`, never `lang-routes`, because the pairs stay tier C (crossplane's text joined to Compose) and would
hold the xref family below B; the include edges are the reading's only xref truth (13 pairs).
Stretch toward A, not built (LB-Q-nginx-rest): an include edge attested by ablation (remove the `include`, `nginx -T`
drops exactly the files the oracle says it loads) and a proxy_pass-to-upstream reference attested by removing the
upstream block (`nginx -t` then reports `host not found in upstream "<name>" in <file>:<line>`).

## Install (scratch, on the external volume)

```sh
python3 -m venv <scratch>/venv && <scratch>/venv/bin/pip install -r calibration/oracles/nginx/requirements.txt
# nginx 1.31.6 (the cross-check that earns tier B): brew install nginx (nginx 1.31.6)
```

## Run

```sh
<scratch>/venv/bin/python calibration/oracles/nginx/nginx_oracle.py -subject <checkout> \
    -out workspace/tmp/oracle/nginx-<subject>.json -routes-out workspace/tmp/oracle/nginx-<subject>-routes.json \
    -nginx /opt/homebrew/bin/nginx -scratch <scratch dir> -files-out workspace/tmp/oracle/nginx-<subject>-files.txt \
    [-summary-out <file>]
go run ./cmd/indexbench lang-oracle -dialect nginx -subject <checkout> \
    -tool-oracle workspace/tmp/oracle/nginx-<subject>.json -only "$(paste -sd, <files.txt>)" -out docs/bench/readings/lang/nginx.json
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

| `nginx` | B (definitions, references, cross_references; C without `-nginx`); A (nginx's verdict on a file, the compose side of routes) | crossplane 0.5.8 `parse(single=True)` per file; `nginx -T` 1.31.6 on a scratch copy of each prefix; `docker compose config` 5.1.4 | definitions (a `package` per file, a `service` per server and upstream, a `route` per location, a `target` per upstream server), `references` (a pass to an upstream of the file), `cross_references` (an include, prefix-relative, file granularity); `-routes-out`: route_references/route_unresolved | `<venv>/bin/python calibration/oracles/nginx/nginx_oracle.py -subject <checkout> -out <json> -routes-out <json> [-nginx <bin> -scratch <dir>]` |

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

The route file is written but, as above, not scored.
