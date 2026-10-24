# proxysql oracle (LB-Q-proxysql)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the proxysql doc dialect.

| Part | Tier | Tool |
|---|---|---|
| the entities the file made: servers, users, query rules, replication hostgroups, scheduler entries, peers, the variables ProxySQL knows; the pairs a server, user, rule or replication entry writes to a hostgroup | A | ProxySQL 3.0.9 itself: `proxysql --initial -f -D <scratch datadir> -c <file>` loads the file and writes `proxysql.db`, read with sqlite3 (`mysql_servers`, `pgsql_servers`, `*_users`, `*_query_rules`, `*_replication_hostgroups`, `scheduler`, `proxysql_servers`, `global_variables`) |
| every line, which keys a file writes, the top-level sections and settings | scanner | a libconfig scanner written in `proxysql_oracle.py` (never fathom's decode); ProxySQL keeps no row of a section or a setting, so those are the scanner's alone and counted in `disclose` |

The file is labelled tier A: ProxySQL decides which entities exist and what each names. A file ProxySQL does not load is
left out and counted; the scanner refuses a bare word as libconfig does, so the dialect and the oracle drop the same file.
On sysown/proxysql the oracle drops 17 variables the dialect reads (`mcp_*` and `genai_*` groups of `src/proxysql.cfg`,
features the release binary has no variables for, and `debug`); they are the `extra definition: 17` that holds L1 at good.

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
