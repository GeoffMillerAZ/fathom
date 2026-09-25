# Actionable quality-work selection

## Bounded task and acceptance

Packet: R1/R5, owner-funded adoption maintenance. An adopting agent selecting a
capability must distinguish untracked quality work from work hidden by its scope.
The current empty `items` array and generic claim boundary do not provide that
distinction or an actionable continuation. This is a guidance gap, not evidence
that a gate has falsely passed.

Inputs: app-kit `4277048e73bf9a497f1d9ac4e05a1dab69c1f949`, existing CUE inventory,
catalog, shared quality reader and MCP/Gallery JSON projections. The worktree was
clean and the original stash was preserved before this task. No consumer repository,
installed binary, host registration, licensed source or quality status is changed.

Add one bounded `selection` object to list output: normalized scope, optional exact
capability ID, `matched` / `filtered-out` / `untracked` status, available scopes and
next action. Available scopes derive from matching inventory items before filtering;
unknown IDs remain errors. Empty inventory also means untracked, never accepted.
Matching records remain authored planning, not verified readiness. Existing list
fields, content revisions, exact-detail lookup and response byte limits stay intact.

Acceptance: synthetic core-only, on-demand-only, mixed, unrepresented and empty
inventory cases; deterministic detached results; unchanged unknown-ID/scope refusal;
actual MCP roundtrip and Gallery JSON parity with actionable empty results. Static
guidance routes an untracked need through the existing JIT/contribution process; it
does not create work automatically or authorize a consumer upgrade.

Authority: existing app-kit implementation/commit/push scope. Non-goals: browser or
human certification, installed-host refresh, UI appearance changes, provider calls,
profile generation, new tools/servers and automatic readiness promotion. ADR-0066
and ADR-0069 govern the continuation; no new architecture decision is needed.

Budget: one frozen regression run before repair, one after repair, then focused
package/race/protocol checks and existing push gates. No browser acquisition. Retain
all failed inputs before editing; a further run requires a diagnosed fixture/code
change, not an unchanged retry. Stop after this selection contract and guidance pass.

## Evidence

The private packet is `/Volumes/m2ext/appkit-evidence/appkit-quality-selection.DGjpOVJn`. Its
`baseline.tar` retains the complete tracked baseline; each exclusive run directory
retains changed/untracked source overlays, the executed runner, source/tool identities,
arguments, environment, stdout/stderr and result. All three runs reported no source
drift. Restore the baseline plus that run's overlay to reconstruct its source, not
the later assessment text. Dependencies and caches were reused, not freshly rebuilt;
this is package/protocol verification, not clean-delivery or performance proof.

| Run | Result |
|---|---|
| `red` | Selection assertions fail against the old implementation in reader and actual MCP/Gallery responses; invalid-input refusal still passes. |
| `green` | All five focused test functions and three protocol subcases pass with unchanged regression assertions. |
| `regressions` | Full race-enabled `internal/qualityplan`, `cmd/appkit-mcp` and `apps/gallery` package suites pass, including the added direct returned-slice ownership test. |

Focused command: `go test -mod=readonly -count=1 -timeout=120s ./internal/qualityplan
./cmd/appkit-mcp -run 'TestListSelection|TestQualitySelectionGuidanceRoundTrip' -v`.
Regression command: `go test -mod=readonly -count=1 -race -timeout=120s
./internal/qualityplan ./cmd/appkit-mcp ./apps/gallery`.
Both use the recorded offline dependency settings and exact local Go selection.
All 189 relative link paths in the changed guide, changelog and this assessment resolve;
`git diff --check` passes. This is a scoped link check, not a full intent audit.

| Retained artifact | SHA-256 |
|---|---|
| `baseline.tar` | `837a9689983e9bab19baf2bb697022f1c85d0ee9cb624e76ddad1e66befb226d` |
| `red/result.json` | `7cd167d37f4e564fa91b8798fd830d63623932825530d2847adab82451eea80a` |
| `green/result.json` | `8425f6b6455db7422d94c1b53d298d80cc9576220e84d675bb00b51f11bbd6c1` |
| `regressions/result.json` | `70f514453c471d1cd48c29b66b32e9e43695df234c13851be27eb869905c7ab7` |

The reader now reports the normalized selection and available scopes independently
of its filtered item list. An untracked capability directs the adopter to the existing
JIT/contribution documents with the exact query, task/profile and revision. A filtered
selection names the scope that actually contains authored work. `matched` means only
that records were returned. Unknown IDs still fail, and a subsequent valid MCP request
works in the same session. Existing item fields, CUE revisions, claims and byte limits
are unchanged; the new metadata participates in the existing size check.

An independent scoped review found no blocking issue. Its suggestion to test direct
Go slice ownership was added before the full regression run; the JSON projection
test alone is not described as ownership proof.

## Contribution and delivery boundary

`APPKIT-QUALITY-01`: originating task is app-kit adoption discovery through its MCP
and Gallery JSON, ownership is the shared quality reader, disposition is returned.
The reusable result is actionable coverage metadata and synthetic refusal/parity
regressions. No consumer fork, installed-host trial or second-app propagation is
claimed. The owning JIT guide and changelog carry the additive-response migration.
No inventory gate or original G1/G4 acceptance criterion is promoted by this repair.

The additive response needs a rebuilt server. Until then, a missing `selection`
field is unknown coverage, not acceptance. Installed delivery and a fresh Claude/Codex
invocation require a separately reviewed refresh; no registration, global config,
consumer pin or installed executable changed in this task.
