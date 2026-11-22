### Fixed

- TypeScript dependency reads now require repository-view evidence ruling out a local package identity before treating a bare import as external. Supported local resolution takes precedence. A local identity with an unresolved target, or an inventory that cannot prove absence, leaves a typed unknown and withholds completeness even when a version is pinned. Repository inventory is reused within each batch. JavaScript behavior is unchanged.
