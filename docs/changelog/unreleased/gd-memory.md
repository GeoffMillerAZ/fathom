### Changed

- Cold plan verification and symbol context use less memory by loading only required graph evidence and projecting stored symbols directly through the scoped store interface.
- File-only and spec-only plans no longer report the typed-fallback partial or `graph_fallback`, and graph-load errors no longer fail them. The spec-only building message names the symbol snapshot.
- Symbol context treats evicted typed edges as non-resident even when its combined graph is cached, so its cold path can run to reload typed details. It leaves the Syntax and Resolved graphs cold, so a later tool pays for those builds.
