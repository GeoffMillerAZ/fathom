# pgbouncer — provenance and divergences

LB-Q-pgbouncer, design `docs/design/language-breadth.md` §4 and §13. A doc-engine dialect (no `tags.scm`, no grammar, no
incumbent: a `pgbouncer.ini` was an unread `.ini`). The fixtures are written for this dialect from the file format
PgBouncer documents (pgbouncer/pgbouncer `doc/config.md`, ISC); nothing is copied from its source, and every host is a
reserved example name.

## How a file is claimed

Manifest basename `pgbouncer.ini`, globs `**/pgbouncer*.ini` and `**/pgbouncer/**/*.ini`, and a sniff on any other
`.ini` (a `[databases]` or `[pgbouncer]` section header in the first 16 KiB). The gateway hub (`fusion/gatewaydispatch.go`)
claims a file only when the doc spec's `content_sniff` also holds, so an `.ini` of another tool stays unread.

## The decode

`docdecode_pgbouncer.go`, the `pgbouncer` decode: a section is a map of its entries; an entry of `[databases]`,
`[users]` and `[peers]` is a map of its connect string's parameters (bare or `'quoted'` values, `''` for a quote), an
entry of `[pgbouncer]` a scalar. A `%include` line is dropped (the file it names is not read), as is any line before
the first header.

## What it models

- **Sections** (`PgBouncerSection`, `section`), named by the header.
- **Databases** (`PgBouncerDatabase`, `target`), named by the entry, `*` the fallback; signed by `host`, `port` and
  `dbname` (never a password).
- **Peers** (`PgBouncerPeer`, `service`), **users** (`PgBouncerUser`, `element`) and **settings** (`PgBouncerSetting`,
  `key`, [pgbouncer] only).
- **References:** a database's `auth_dbname` parameter and the `[pgbouncer]` section's `auth_dbname` setting name a
  database the file declares (`declared_only`).

Every entity is `sensitive`: a connect string and a setting may hold a password.

## Not modelled (beyond the L2 target)

`host` as a cross-file service reference (compose service names, hosts), `%include` as a file edge, and the
`auth_file` userlist.
