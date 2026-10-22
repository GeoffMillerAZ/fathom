### Fixed

- Certification scans honor corpus exclusions and stop when cancelled or refused. Unresolvable test declarations no longer force recompilation of unchanged corpora.
- Reviews share typed edges and call graphs with the daemon's memo, reducing repeated allocations without changing review results.
- Oversized Go files no longer abort certification scans. Ingest summaries and daemon logs report skipped oversized files, and daemon certification refresh failures are logged.
- Certification gates compare claim identities, settle duplicate markers, and compile newly indexed declared tests.
- Certification ingests preserve test-file citations across recompiles and sweep existing claims when declarations or watermarks change, marking moved tests stale and vanished tests contradicted.
- Reviews preserve typed impact when an optional memo graph cannot be loaded or built.
