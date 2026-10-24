# pgbouncer oracle (LB-Q-pgbouncer)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the pgbouncer doc dialect.

| Part | Tier | Tool |
|---|---|---|
| the entity set: databases (`*` included), peers, users, [pgbouncer] settings | A for membership | PgBouncer 1.26.1 itself, started on a scratch instance with the file's own sections and its machine-naming settings replaced; read over its admin console (`SHOW DATABASES`, `SHOW PEERS`, `SHOW USERS`, `SHOW CONFIG`) |
| every line, and the `auth_dbname` pairs | C | a line scanner written in `pgbouncer_oracle.py` (never fathom's decode); the console does not show a per-database `auth_dbname` |

The file is labelled tier C because its pairs and lines are the scanner's: the level is capped at good (design §1.3). The
agreement of the scanner's entity set with PgBouncer's is counted in the file's `disclose` and on stderr; a file PgBouncer
refuses (a parameter its version does not know) is read by the scanner alone and counted.

## Install (scratch)

PgBouncer 1.26.1 (ISC) built from the tag the subject is pinned at (`pgbouncer_1_26_1`, with its `lib/usual` submodule):
`./autogen.sh && ./configure --prefix=<scratch>/pgbouncer-1.26.1 --without-cares --with-openssl && make && make install`
(needs libevent, OpenSSL, autoconf, automake, libtool, pandoc). Debian and Ubuntu's `apt install pgbouncer` (1.22) also
works but refuses a few parameters the 1.26 test configurations use. `psql` is needed on PATH (postgresql-client). As
root the instance runs `-u postgres`. Python 3 standard library only.

## Run

```sh
python3 calibration/oracles/pgbouncer/pgbouncer_oracle.py -subject workspace/calibration/pgbouncer/pgbouncer \
    -pgbouncer <scratch>/pgbouncer-1.26.1/bin/pgbouncer -out workspace/tmp/oracle/pgbouncer.json
```

For `task lang:loop`, a `runners.json` keyed by the queue's oracle name:

```json
{"pgbouncer config load on a scratch instance (tier A) or Python configparser (tier C)": {"command": "python3",
 "args": ["calibration/oracles/pgbouncer/pgbouncer_oracle.py", "-subject", "{subject}", "-out", "{out}", "-pgbouncer", "<scratch>/pgbouncer-1.26.1/bin/pgbouncer"],
 "format": "tool"}}
```
