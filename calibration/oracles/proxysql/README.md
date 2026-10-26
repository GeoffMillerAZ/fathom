# proxysql oracle (LB-Q-proxysql)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the proxysql doc dialect.

| Part | Tier | Tool |
|---|---|---|
| the entities the file made: servers, users, query rules, replication hostgroups, scheduler entries, peers, the variables ProxySQL knows; the pairs a server, user, rule or replication entry writes to a hostgroup | A | ProxySQL 3.0.9 itself: `proxysql --initial -f -D <scratch datadir> -c <file>` loads the file and writes `proxysql.db`, read with sqlite3 (`mysql_servers`, `pgsql_servers`, `*_users`, `*_query_rules`, `*_replication_hostgroups`, `scheduler`, `proxysql_servers`, `global_variables`) |
| every line, which keys a file writes, the top-level sections and settings | scanner | a libconfig scanner written in `proxysql_oracle.py` (never fathom's decode); ProxySQL keeps no row of a section or a setting, so those are the scanner's alone and counted in `disclose` |

The file is labelled tier A: ProxySQL decides which entities exist and what each names. A file ProxySQL does not load is
left out and counted; the scanner refuses a bare word as libconfig does, so the dialect and the oracle drop the same file.
The original regular 3.0.9 binary leaves 17 definitions unmatched. A matching debug binary and a successfully
built source-matched PROXYSQL40 debug instrument attest debug, leaving 16. Two are GenAI placeholders. The 14 MCP
literal names have an extra `mcp_` compared with the native registry; controller Q5 retains the strict contract and 16 genuine misses; no alias
normalization or adjudication changes the recipe. See the dated assessment and committed registry comparison.


## Install (scratch)

The release package `proxysql_3.0.9-ubuntu24_amd64.deb` (sha256 `a20548b17ede6d9e014decb1eefb1cb0775f643932561388770d2f50c8d316ab`)
from github.com/sysown/proxysql/releases, unpacked with `dpkg-deb -x` (no install; the binary is `usr/bin/proxysql`). Python 3
standard library only (sqlite3).

## Run

```sh
python3 calibration/oracles/proxysql/proxysql_oracle.py -subject workspace/calibration/proxysql/proxysql \
    -proxysql <scratch>/proxysql-3.0.9/proxysql -work <scratch dir> -out workspace/tmp/oracle/proxysql.json
```

For `task lang:loop`, a `runners.json` keyed by the queue's oracle name:

```json
{"proxysql --initial on a scratch datadir (tier A) or python libconf (tier C)": {"command": "python3",
 "args": ["calibration/oracles/proxysql/proxysql_oracle.py", "-subject", "{subject}", "-out", "{out}", "-proxysql", "<scratch>/proxysql-3.0.9/proxysql", "-work", "<scratch dir>"],
 "format": "tool"}}
```

## Source-matched debug and registry reproduction

The following reproduction targets a separately provisioned Linux environment.
The completed build and runtime scratch were removed after measurement.
All source/build artifacts remain in the selected scratch root. Run one heavy job at nice 10,
starting only when one-minute load is at most 6. Containers expose no ports;
native measurements use `--network none`. Only configuration loading and admin
variable-name queries execute; no backend, pipeline, policy or model is invoked.
All code and source identities precede their recorded measurements.

From the repository root with the clean pinned subject at
`workspace/calibration/proxysql/proxysql`:

```sh
mkdir -p workspace/tools/proxysql workspace/tmp/proxysql-source
# A Git clone preserves upstream's tracked symlinks without host copy semantics.
git clone --no-hardlinks workspace/calibration/proxysql/proxysql workspace/tools/proxysql/source40
image=proxysql/packaging@sha256:c6bf0802f14349c00f3f935c237591f897d1a3c12ce3943cf8d7a6a2fc3067c5
docker pull --platform linux/arm64 "$image"
docker run --rm --platform linux/arm64 --cpus 2 --memory 3g \
  -v "$PWD:/repo:ro" -v "$PWD/workspace/tools/proxysql:/probe" "$image" \
  sh /repo/calibration/oracles/proxysql/source_build.sh
docker build --platform linux/arm64 -t fathom-tools-proxysql-source-probe:ubuntu24 - < calibration/oracles/proxysql/Dockerfile.source-probe
docker run --rm --platform linux/arm64 --cpus 2 --memory 3g --network none \
  -v "$PWD:/repo:ro" -v "$PWD/workspace/tools/proxysql:/probe" \
  fathom-tools-proxysql-source-probe:ubuntu24 nice -n 10 sh /repo/calibration/oracles/proxysql/linux_probe.sh source-measure
docker run --rm --platform linux/arm64 --cpus 2 --memory 3g --network none \
  -v "$PWD:/repo:ro" -v "$PWD/workspace/tools/proxysql:/probe" \
  fathom-tools-proxysql-source-probe:ubuntu24 nice -n 10 python3 /repo/calibration/oracles/proxysql/registry_probe.py \
  -binary /probe/source40/src/proxysql -plugin /probe/source40/plugins/genai/ProxySQL_GenAI_Plugin.so \
  -client /usr/bin/mariadb -work /probe/registry-native -out /probe/registry-native.json
python3 calibration/oracles/proxysql/compare_registry.py -subject workspace/calibration/proxysql/proxysql -registry calibration/oracles/proxysql/native-registry-7ddb3dc.json
GOWORK=off go build -o workspace/tools/indexbench ./cmd/indexbench
workspace/tools/indexbench lang-oracle -dialect proxysql -subject workspace/calibration/proxysql/proxysql -subject-name sysown/proxysql \
  -tool-oracle workspace/tools/proxysql/source-4.0.9-debug-oracle.json -out workspace/tmp/proxysql-source/reading.json -unmatched workspace/tmp/proxysql-source/unmatched.json
workspace/tools/indexbench lang-retime -reading workspace/tmp/proxysql-source/reading.json -runs 3 -write -subject sysown/proxysql=workspace/calibration/proxysql/proxysql
```

Start with fresh scratch/profile/reading paths: native `--initial` state and
`lang-oracle` merge existing artifacts. The registry profile is authored and
separate; it is **not** inserted into the canonical corpus or native truth.
Only names are emitted. The native registry has 14 MCP and 32 GenAI names;
`compare_registry.py` reports literal and hypothetical shortened names without
changing grading. None of the 16 literal names matches; dropping `mcp_` would
match the 14 underlying MCP names, but is not a native alias in this contract.

Published regular/debug/4.x asset digests and release targets are in
`candidates-t-r2.json`. The named candidate/version/result audit lives in
`docs/assessments/2026-10-25-tools-proxysql.md`; source, image, artifact hashes
and wrapper identities are in `source-build-7ddb3dc.json`. The nonmatching 4.0.10
partial run is excluded from grading; the matching full build succeeds.
Initial scratch issues (lightweight-tag version parsing, bind-mount archive
metadata and Unix sockets) were corrected in the recipes, not recorded as
intrinsic source-build ceilings. The original source stays tracked-clean.

For the regular/debug debs, build `Dockerfile` for `linux/amd64`, fetch the
exact asset from `candidates-t-r2.json` into the scratch root mounted at
`/probe`, then invoke `linux_probe.sh measure LABEL PACKAGE` in that offline
image. The script verifies the published SHA256 before unpacking or execution.
Native 3.0.9 regular reports 356 definitions; debug reports 357, both 10 references.
