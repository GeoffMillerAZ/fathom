### `memory` — removed

Anchored memory (`fathom memory add|list|review|show`, the `memory_search` MCP tool
and the edit hook's memory block) was removed by SR-13 (surface reckoning F-45,
ADR-053): 0 memories in any repository. Continuity threads (`fathom thread note`)
carry what the next session must know. A `<repo>/.fathom/memories.jsonl` left from
before is no longer read; `fathom purge` fails naming it so a quoted secret in it
is never reported purged. Migration: `docs/upgrades/0.9.0-surface-reckoning.md`.
