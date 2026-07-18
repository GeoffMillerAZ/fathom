# Reference jump-off for `toolchains/materialization/workspace-query-builder`

These are **faithful reconstructions of the original scanned source** (workspace-query-builder.go),
the most complex components in the deck. They are partial scans — `// [scan gap: ...]`
markers indicate un-photographed line ranges — so they do **not** compile as-is and
are deliberately parked in this `_reference/` directory (Go ignores `_`-prefixed dirs).

Use them as the design jump-off when implementing `toolchains/materialization/workspace-query-builder`: port the structure,
types, and logic; fill the scan gaps from the design docs in
`tmp/design_inspiration/files/`. Do not import this directory.
