### Fixed
- The CUE typed pass and the embedded schema loader no longer build the default module registry, so ingesting a CUE module with external dependencies cannot contact the public registry. Such imports report as failed packages, and the warning names the failing package and the dependency.
- Known limit: one unresolvable external import fails the CUE typed pass for the whole module, not only the importing package, so modules whose external dependencies used to be fetched lose their CUE typed edges (syntax-only edges remain, and the failure is counted).
