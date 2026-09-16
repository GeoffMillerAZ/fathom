## Claims custodial pass, 2026-09-16: what this run found

### Verdict

Every unproven-claims row this pass touched (U12 — "claims done but pins no named
test") was already correctly done; the code was never wrong, the rows were just
missing the `reachable_from:` annotation the grammar wants. All 104 non-`verbs.md`
rows verified `test_reaches: pass` before this pass and still verify `pass` after
pinning — nothing was unclaimed, because nothing failed reachability. `verbs.md`
was left alone (other workers own it); its 7 unproven rows are listed below for
whoever picks that file up next.

### Before / after (doctor's `unproven_claims`, service `fathom`, same daemon)

| Kind | Meaning | Before | After | Delta |
|---|---|---:|---:|---:|
| U12 | claimed `[x]` on a sensitive surface, no `reachable_from` | 111 | 7 | −104 |
| U13 | sensitive surface has no `guard.cue` provenance entry | 15 | 15 | 0 (out of scope — not a plan-row defect) |
| U2  | claimed `[x]` with no acceptance contract at all | 4 | 4 | 0 (out of scope — needs a contract, not a test pin) |
| U3  | invariant I35 restated wrong | 1 | 1 | 0 (out of scope — not a plan-row defect) |
| **Total** | | **131** | **27** | **−104** |

(The task brief's "127 [U12 111, U13 15, U3 1]" undercounts the live daemon read by
the 4 U2 rows, which were already present at base `e184171` — not new drift. U13,
U2 and U3 are real audit findings but none of them are "claimed row missing a test
pin"; none are fixable by pin-or-unclaim, so this pass left them for their own
remedies (`fathom intent export --guard`, adding an acceptance contract, and fixing
`docs/intent/invariants.md` I35, respectively).)

### Method

For each U12 row, `fathom verify --service fathom --plan <plan> --json` already
reports a `test_reaches` criterion at `status: pass` naming one confirmed example
("N test(s) statically reach it, e.g. fathom::pkg.TestName") — the call graph's own
answer, not a guess. That example test name was pinned verbatim as
`reachable_from: TestName`, spot-checked against `mcp__fathom__test_impact` for
`registerServerStats` (A1-1), which independently listed `TestAdoptionServiceScoped`
among 722 confirmed reaching tests. No test name was invented; no row's `{symbol,
package}` contract was touched — only `reachable_from` was added, via
`verify.UpsertAnnotationField` (the real plan-grammar editor), copying the exact
`reachable_from: TestName` form already used elsewhere (e.g. `ambient-fathom.md`).

### Pinned, by plan (104 rows, 29 files)

| Plan | Rows pinned | Plan | Rows pinned |
|---|---:|---|---:|
| expert-suite-q.md | 12 | reach-parity.md | 3 |
| receipts.md | 9 | review-verb.md | 3 |
| bench-pyramid.md | 7 | store-sqlite.md | 3 |
| strengths-bench.md | 7 | surfaces-week1.md | 3 |
| custodian.md | 5 | toolident.md | 3 |
| expert-suite-r.md | 5 | trust-and-tokenomics.md | 3 |
| gke-module-host.md | 5 | expert-depth.md | 2 |
| m1-manifest.md | 5 | gap1-instrumentation.md | 2 |
| layer-catalog.md | 4 | lang-truth.md | 2 |
| proof-credibility.md | 4 | sdd-reconciler-v0.md | 2 |
| blast-depth.md | 3 | activation.md | 1 |
| findings-tracer.md | 3 | expert-suite-o.md | 1 |
| pulled-tools-excellence.md | 3 | extraction-treesitter.md | 1 |
| | | new-use-cases.md | 1 |
| | | o8-precision-and-speclines.md | 1 |
| | | pack-taxonomy-adoption.md | 1 |

Test names used (all confirmed reaching, none invented): `TestAdoptionServiceScoped`
(65), `TestAnsweredBeatCarriesDisplacedCount` (21), `TestBuild_PropagatesListError`
(11), `TestCmdFindingsStatus_JSON` (2), `TestBind_DriftedModule_FlagsRules` (2),
`TestAggregateBundleInput_HashesServiceNames` (1),
`TestInstallPack_MidCopyFailureLeavesNoDebris` (1), `TestCompile_HappyPath` (1).

### Unclaimed

None. Every one of the 104 rows verified `test_reaches: pass` before this pass
(the prior worker's finding held for all of them, not just a sample); there was no
row where the call graph found zero reaching tests, so the "unclaim" branch of the
custodial procedure was never exercised this round.

### `verbs.md` — left untouched, 7 unproven U12 rows for the next worker

| ID | Symbol / package |
|---|---|
| VB-1 | `promotedVerbs`, `internal/mcp` |
| VB-2 | `registerSafeDeleteCheck`, `internal/mcp` |
| VB-8 | `rankByGoverningScope`, `internal/semantic` |
| VB-9 | `obligationLayers`, `internal/mcp` |
| VB-10 | `renderObligationReceipt`, `internal/mcp` |
| VB-26 | `registerLocalize`, `internal/mcp` |
| VB-28 | `auditCodeFactSentences`, `internal/custodian` |

### Gate

`nice -n 10 go test -race ./cmd/fathom/... ./internal/verify/... ./internal/intent/... -run 'Plan|Plans|NoOrg' -count=1`
— all green (`internal/plans` does not exist in this repo; the plan-grammar tests
that actually match `grep -rl 'docs/plans' --include='*_test.go'` live in
`cmd/fathom`, `internal/verify` and `internal/intent`).

Every touched plan re-verified individually at exit 0
(`fathom verify --service fathom --url http://127.0.0.1:7575 --plan docs/plans/<file>.md`),
and all 104 pinned rows individually confirm `status: pass` post-pin — no
regressions introduced.
