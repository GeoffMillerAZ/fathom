### Added

- PgBouncer configuration (`pgbouncer.ini`, and any `.ini` that opens a `[databases]` or `[pgbouncer]` section) extraction at L2 (good), measured against PgBouncer 1.26.1's own load on pgbouncer/pgbouncer: sections, databases, peers, users and settings, and `auth_dbname` as an in-file reference to the database it names.
