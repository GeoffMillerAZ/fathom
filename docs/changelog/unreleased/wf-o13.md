### Fixed

- Certification scans honor corpus exclusions and stop when cancelled or refused. Unresolvable test declarations no longer force recompilation of unchanged corpora.
- Reviews share typed edges and call graphs with the daemon's memo, reducing repeated allocations without changing review results.
