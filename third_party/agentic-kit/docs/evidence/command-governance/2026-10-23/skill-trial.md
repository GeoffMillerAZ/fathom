# Command safety guide-only trial, 2026-10-23

I followed `docs/command-safety.md` and `.claude/skills/command-safety-audit/SKILL.md` with the offline binary at `/Volumes/m2ext/tmp/agentic-kit-build/command-lab`. Inputs were only the supplied synthetic fixtures plus scratch copies of their versioned JSON controls. I did not inspect private logs, invoke a provider, run a proposed command, install anything, or change hooks. All five runs emitted `noActionAuthority:true` and no stderr.

| Case | Exit | Exact decisive output |
|---|---:|---|
| Supplied synthetic manifest | 0 | `status:"complete"`, `expectedSources:3`, `loadedSources:3`, `bytes:392`, `records:5`; nonzero proposals: `build:1`, `git:1`, `read:2`; nonzero candidates: `search:1`; all failures zero. |
| Supplied read-range request | 0 | `status:"advisory"`, `effectiveAllowed:["read-range","search-literal","git-status"]`, `effectiveMaxBytes:2048`, `effectiveMaxResults:10`, `codes:[]`, `classifierStatus:"unknown"`. |
| Scope copy requiring absent `missing` layer | 2 | `status:"incomplete"`, `code:"layer_set_mismatch"`. |
| Request copy with path `../synthetic-private` | 1 | `status:"blocked"`, `codes:["path_traversal"]`, `classifierStatus:"none"`. |
| Manifest selecting `codex-unsupported.jsonl` | 2 | `status:"incomplete"`, `expectedSources:1`, `loadedSources:0`, `bytes:102`, `records:1`, `failures.unsupported:1`; proposal and candidate counts zero. |

The guide was sufficient to reach a useful offline result and distinguish complete declared-source parsing, advisory eligibility, deterministic block, and incomplete coverage. Counts are proposals and literal candidates, not executed commands. The selected fixtures do not represent complete user history, and a caller-supplied policy does not prove organizational authority. This trial does not qualify a runtime gate or provider integration. The security re-review still finds an aliased dynamic JS coverage gap and line-cap overread in the current binary.

## 2026-10-23 current guide rerun

The guide now selects `manifest-direct.json` for its complete example. A freshly built binary returned exit 0, `status:"complete"`, 3 expected and 3 loaded sources, 349 bytes, 5 records, nonzero proposals `build:1`, `git:1`, `read:2`, `search:1`, zero candidates and zero failures. The separate `manifest.json` contains a JS wrapper: exit 2, `status:"incomplete"`, 3 expected and 2 loaded sources, 392 bytes, 5 records, nonzero proposals `build:1`, `git:1`, `read:2`, one `search` candidate and `unsupported:1`. Both reports retained `noActionAuthority:true`; stderr was empty. The earlier open F1/F6 statement above is historical; `final-review.md` records their closure in the current code.
