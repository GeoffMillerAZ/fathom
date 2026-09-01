# Scope

Where the edges are right now. `docs/roadmap.md` holds the subscriber-gated sequence;
`TRACKING.md` holds every capability's checkbox; this file is the current position with
exit tests. Done goals are deleted, not archived.

## Current goals

**G1. Ratify D4: does the kit go public, and is it a 1.0 candidate?** Done when: Geoff
records the decision (`decisions/`), which unblocks H8 Stage B
(`docs/plans/handoff-h8-public-candidacy.md`, 5 open rows). Status: blocked on Geoff
since the uplift plan closed at v0.6.0.

**G2. Settle `kits/guardrails-pipeline`'s classification.** It is `experimental` with a
live production consumer (throughline), so that consumer has no compatibility promise.
Done when: promoted to `stable` in `compat.md` with the promise, or the risk is recorded
as accepted. Status: open, flagged in `compat.md` §75–83.

**G3. Close the open `TRACKING.md` rows.** F8 (guardrails-pipeline structured findings),
F9 (EncryptedVault deterministic minting), H8-1 (`CallTool` pre-call pin verification,
Geoff's call), more `scenarios/` blueprints. Done when: each is `[x]` with the gate green
or `[-]` with a disposition. Status: 5 open, 3 in progress, 276 done.

**G4. Apply the change-records budget from the next entry on.** Done when: every new
`CHANGELOG.md` line is at most two sentences with a link, and `[Unreleased]` is rolled
before the next tag. Status: 12 legacy long entries remain under `[Unreleased]` by rule.

## Current boundaries

- 172 library packages across five roots (`foundations` 148+, `kits` 13, `runtimes` 2,
  `surfaces` 2, `toolchains` 4); 124 `stable`, 25 `experimental`, 24 `internal-ish`
  (`docs/compat.md`). `cmd/`, `examples/`, `scenarios/`, `specs/` are not library surface.
- Zero external dependencies; two nested modules (`kits/docextract/{pdf,xlsx}`) with
  their own tags. Go 1.26, CUE 0.16.
- Three named consumers checked by `task downstream-check`; adoption is sibling
  checkout + `go.work` + `require`/`replace` (`docs/adopting.md`).
- LLM adapters: anthropic, bedrock (+awscred), openai (+Azure), claudecli, codexcli.
- Latest tag `v0.7.0` (2026-07-16); `[Unreleased]` holds 12 entries, `Release impact: minor`.

## Compatibility surface

Changes to these are breaking: `!` on the commit, `### Breaking` in `CHANGELOG.md`, a
migration note in `docs/upgrades/` when the fix exceeds one sentence, and the same-commit
`TRACKING.md` update `compat.md` demands.

- Exported Go API of every `stable` and `internal-ish` package under the five roots.
  Evidence: `docs/compat.md` §Promise.
- Typed-error sentinels of `stable` packages (`errors.Is`/`errors.As` behavior).
- `schema.cue` of `stable` packages (fixtures included).
- Nested-module tags and the `go.work` layout consumers depend on. Evidence: `compat.md`
  §v1 decision record.
- `cmd/constitution-lint` check semantics: consumers inherit the kit's gate expectations
  through `fathom-pack`.

Pre-1.0: a breaking change to `stable` still bumps minor and still gets every line above.
`experimental` may change, rename, or retire at any time; a `CHANGELOG.md` line is still
written.

## Non-goals

Long-lived "what the kit is not" (framework, provider lock-in, research playground,
compliance checkbox) lives in `VISION.md`. The ones an agent is most likely to re-propose:

- **A sandbox or isolation claim for `bounded-exec`.** Why: confinement is not
  isolation; stdlib cannot deliver it. Revisit when: never; consumers add isolation.
- **A deletion story for data written outside the tokenization vault.** Why: journals,
  event stores, and cassettes are append-only by design. Revisit when: a consumer's
  retention regulation names a specific store; then it is a decision, not a feature.
- **Runtime per-call capability re-checking for the trifecta mechanism.** Why: a
  deliberate seam, not an oversight (`threat-model.md`). Revisit when: a consumer
  reports a capability changing mid-run.
- **An executor for `coordination/*`.** Why: uplift D7; plan-only until a real fan-out
  workload subscribes. Revisit when: `futures.md` F8's trigger fires.
- **The deprecated MCP HTTP+SSE transport.** Why: removed from the 2025-11-25 spec and
  absent from Claude Code's client. Revisit when: never.
- **AssumeRole / role-chaining in `bedrock/awscred`.** Why: no named consumer routes
  through it (D8). Revisit when: a consumer names the need.
- **A dependency, however small.** Why: I1 is constitution, not marketing (uplift D1).
  Revisit when: never; the WASM lane and the zero-conflict promise depend on it.
- **Porting `_reference/` sources wholesale.** Why: mine, don't port (uplift D8).
  Revisit when: never.
