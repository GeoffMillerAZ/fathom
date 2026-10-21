### Fixed
- Typed enrichment source-loads the selected roots and their dependency closure once per module load, avoiding redundant export compilation while retaining test variants and full type information.
- Dependency type errors are disclosed before a selected package's cache hit, with bounded deterministic diagnostics and preservation of its last good edges.
