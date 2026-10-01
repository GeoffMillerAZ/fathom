## Module host (no MCP tools)

The four module MCP tools (`list_modules`, `module_obligations`, `module_query`,
`module_staleness`), the `/rpc/modules` surface and hosted-modules mode
(`--modules-token-env`) were removed by SR-17 (surface reckoning F-68, ADR-053) —
the `module_query` wire contract v0 with them. The module system stays: the Expert
Module v0 manifest, `schema/module`, `.fathom/modules.cue`, `internal/modspec`, the
`fathom module` verbs, and `serve-http --module <path>` (repeatable), which mounts
conforming expert-module stores **READ-ONLY** so `obligations` unions in the rules of
every module a service binds (docs/design/platform-modules.md §3.3). Mount
conformance is checked at start against store-format-v0.md §2's markers
(`application_id == 0x46415448` AND `user_version >= 1`); a non-conforming or
unreadable store **aborts serve-http startup loudly**, and every mount opens
`file:<path>?mode=ro`. Migration: `docs/upgrades/0.9.0-surface-reckoning.md`.

---
