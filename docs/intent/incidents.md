# Incidents

Every incident yields an invariant, a test, or a decision and, once incident maps exist, the
affected path's map (*planned extension, not in force*: `docs/plans/exploration-maps.md` XM-26
and XM-29, gated on live maps XM-D1; the map id then goes in the Outcome). Open records are
≤10 lines; closed ones compress to one line. The full friction catalogs stay where they are and are
not copied here: `docs/dogfood-observations.md` (O1–O29, root-cause narratives) and
`.fathom/observations.md` (O1–O19, `fathom observe` intake). Only incidents with a
consequence for intent are recorded below. Rationale: `agentic-kit/docs/project-intent/operating.md` §2.

Classes: gate-bypassed · wrong-done-claim · intent-violated · platform-fact ·
contract-broken · other. Record with `intent-incident`.

## Open

### INC-1 2026-08-25 Stale agent worktrees and branches accumulate
What: 24 checkouts under `.claude/worktrees/` and 22 `worktree-*` branches, the oldest
from July, none owned by a live session.
Evidence: `git worktree list`; `git branch` at bootstrap.
Would have caught it: the parallel-agent protocol's cleanup rule (operating.md §4: a
worktree older than its session is an incident).
Class: gate-bypassed
Outcome: pending (deletion needs the owner: `questions.md` Q3; then `autonomy.md` "Ask first" carries the rule)

### INC-14 2026-08-26 A design declared itself adopted before its decision record existed
What: the first draft of `docs/design/evidence-plane.md` carried "adopted (ADR-026)" and
three intent docs cited ADR-026 while `ADR.md` had no such section; the design panel caught it.
Evidence: panel workflow `wf_3a238f27-885` (law-and-intent lens, blocking finding 1).
Would have caught it: a check that every `ADR-\d+` cited in `docs/design/*.md` status lines
and `docs/intent/**` resolves to an existing record.
Class: wrong-done-claim
Outcome: pending (`docs/plans/evidence-plane.md` EP-14 `adr_references_resolve`; ADR-026 now exists)

### INC-23 2026-09-26 The daemon took the operator's machine: ~29 GB resident, swap full, a relaunch loop
What: request paths did work proportional to repository and fleet size (obligations' two whole-tree reads per edit, `list_services` loading every symbol row, cross-service scans), multiplied by concurrency (49 cold `list_services` at once), abandoned work (`context.Background()` on every RPC) and inline re-ingest; launchd `KeepAlive true` relaunched it 5 times in 17 min; jetsam killed neighbours; `SQLITE_FULL` on reads rendered as `symbol_count 0`. Same day: 22 h scratch-daemon leaks and tests writing to the operator call log.
Evidence: heap profiles heap-0518 (730.7 GB request-path alloc in 62 min), heap-2118, heap-2315; `JetsamEvent-2026-09-26-135805.ips`/`-140229.ips`; `~/.fathom/logs/fathom-http.{log,err}`; the call log (peak 49 concurrent calls, 05:24:08Z); hotfix 4200482a.
Would have caught it: a declared footprint budget enforced by the daemon, cost-classed admission with cancellation, a 10×-scale cost test per tool and a cold-stampede test counting bytes, and a crash-loop breaker.
Class: other
Outcome: ADR-047; invariants I47–I53; principle P23; `docs/plans/responsible-tenant.md` (pending until its rows prove).

### INC-24 2026-09-22 A memory incident was closed "bounded" on an instrument that cannot fall
What: O108 (swap killed the pre-push gate twice) was closed "mitigated-and-bounded" and O112 "eliminated at the mechanism level" on readings of `HeapSys` (never decreases by definition) and a sequential test counting Discover calls; no invariant, test or decision followed, and the same root cause produced INC-23 four days later.
Evidence: `.fathom/observations.md` O108, O112; `CHANGELOG.md` "bounded at the peaks"; 0 of 193 scavenge lines show `heap_sys` falling by more than 1 MiB.
Would have caught it: this file's own rule (every incident yields an invariant, a test or a decision) applied to resource incidents, and a test that counts bytes under concurrency.
Class: wrong-done-claim
Outcome: I47 (never report `HeapSys` as a footprint), I48 and RT-M1. Corrections RT-138..RT-140 landed 2026-09-27: O108 and O112 are marked superseded by INC-23, the CHANGELOG entry no longer says "bounded at the peaks", and fleet-index.md names `phys_footprint` and `fathom resources`.

## Closed

### INC-22 2026-09-19 Two green gates, no push: the ssh session did not survive the pre-push hook
What: gated pushes 32 and 33 of the substrate round passed every lefthook step (validate 342 s / 162 s, hermetic verify 83 s, review) and then `git push` exited 141 (SIGPIPE) with no transport output; origin/main stayed at d8770983. git opens the transport before it runs the pre-push hook, so the ssh session to the remote sat idle for the whole gate (9 min, then 5.5 min with a warmed build cache) and was gone when git sent the pack. ssh keepalives changed nothing.
Evidence: `<scratchpad>/push-full32.log`, `push-full33.log` (lefthook summary green, no `main -> main`); `git ls-remote` unchanged after both.
Would have caught it: pushing over a transport with no session to lose. Push 34, over smart HTTP with the same hook, landed 54bf2d0b.
Class: instrument-dies-after-passing (the gate proved the tree; the delivery failed silently)
Outcome: `task push` sends the pack over smart HTTP derived from the push remote (`scripts/push-http.sh`, never a hardcoded URL — ADR-011); credentials from the GitHub CLI when present. Invariant I41; `scripts/push-http_test.sh` in `task scripts-check`.

### INC-21 2026-09-19 The pre-push verify gate measured the daemon's index of a different tree
What: the first gated push of the substrate round failed its `verify` step with "25 claimed: 0 verified" for `docs/plans/substrate.md` because `bin/fathom verify` used the default store — the daemon's index of the operator's checkout at d8770983 — while the symbols it was asked to prove exist only in the tree being pushed.
Evidence: `<scratchpad>/push-full31.log` ("provenance: daemon binary @ 7dd702ac ≠ this fathom binary @ 3095a5d2 — the store (watermark d8770983) was built and queried by the daemon's extractor").
Would have caught it: a gate that indexes what it verifies; the provenance line already said the two disagreed.
Class: gate-measured-the-wrong-thing (I2/I3 class)
Outcome: the lefthook `verify` step ingests HEAD into `workspace/tmp/gate-verify.sqlite` and verifies against it; an ingest failure is a named fail-open, never a silent pass. ADR-041 decision 10.

### INC-20 2026-09-19 Two pushes reached main ungated after the push clone's hooks were wiped
What: origin/main 7dd702ac (42 commits) and d8770983 (docs) were pushed from the scratch push clone while `.git/hooks/pre-push` (lefthook) and `.git/HEAD` were missing; `task validate` never ran before either push. Both push logs were two lines long.
Evidence: `<scratchpad>/push-full29.log`, `push-full30.log` (2 lines each, no gate output); `.git/hooks` held only samples and `.git/HEAD` was absent at 2026-09-19 00:00; an unknown process touched `.git` at midnight.
Would have caught it: a push script that refuses to push unless the pre-push hook is a lefthook hook; a log-length check that treats a gate-less push log as a failure.
Class: gate-bypassed (I2/I3 class)
Outcome: the full gate was re-run against d8770983 after the fact (result recorded in the ADR-041 round's CHANGELOG entry); `push-internal6.sh` aborted without the hook as an operator-local stopgap, now superseded by the committed gate: `scripts/push-check.sh` (`task push`, `task scripts-check` → `scripts/push-check_test.sh`), invariant I40; ADR-041 decision 10.

### INC-19 2026-09-14 Unreadable call logs still produced numerical Impact costs
What: Impact showed zero response cost for an unreadable log, or stale cost/outcome rows when populated values accompanied the failure.
Evidence: `TestObservatoryImpactCallLogUnavailableSuppressesDerivedCosts` failed behaviorally; app-kit browser fixture also recorded 36 missing-source disclosure failures.
Would have caught it: test the source-availability flag against every derived projection, with both empty and poisoned populated inputs.
Class: contract-broken (I38)
Outcome: `TestObservatoryImpactCallLogUnavailableSuppressesDerivedCosts`, `TestObservatoryImpactCallLogUnavailableWithEmptyRowsUsesSourceReason` and readable-zero/positive canaries; response cost, ratio and tool rows retain exact escaped absence while independent schema measurements survive.

- INC-25 2026-09-28 Wall-clock timing tests flipped under parallel worker load and killed three consecutive 40-minute pre-push gates (the RT-7 whole-hook bound, the CT-5 typed-load guard, the langbench throughput floor) → those three gates now judge CPU time, not wall clock (`07c634b8`, `e7f63140`, `008ac775`; owner decision D1) and agentic-kit's orchestration guidance names gates under load.
- INC-18 2026-09-14 Fleet observation summaries collapsed missing coverage into zero or an exact subtotal; home also zero-filled unreadable call-log/catch evidence (contract-broken, I38). Render-level red tests caught it → shared app-owned coverage projection and `TestObservatoryObservationCoverage*` regressions; source semantics and wire model stay unchanged.

- INC-16 2026-09-01 A third `package main` (`cmd/loctruth`) collapsed `fathom::main.run`, so
  `callgraph.Build` kept the last-ingested `run` and five untouched ambient-fathom rows (SP-4,
  SP-7, SP-8, SP-9, TH-1) were silently REFUTED (O21); caught by the wave-1 integration probe --
  a `--strict` sweep over all 51 plans on a freshly built store -- not by any test → PC-32:
  symbol keys carry the package directory (`service::<dir>/<package>.Name`), `TypedEdgeEpoch` 2
  rebuilds stored typed edges, `TestTwoMainsSameFuncNameNeverCollapse` (f6fda39).
- INC-17 2026-09-01 A worker worktree's post-commit hook re-rooted the daemon's `fathom` service
  at `.claude/worktrees/…` while the MCP `ingest` tool answered `{ok:true}` four times and named
  no root, so every "refreshed" verify read a tree that was not merged main (O22) → PC-33:
  `internal/worktree.CheckReroot` refuses a re-root at a linked worktree of the recorded root on
  BOTH write paths (MCP handler and `pkg/client`), the `ok` envelope carries `root`, the
  post-commit hook in a worktree refreshes nothing and names what it left alone; invariant I30
  extended (cada740).
- INC-13 2026-08-25 The raw-`cp` deploy instruction survived outside the custodian's corpus (`docs/adoption/claude-code.md` ×3, doctor's `binary_fresh` remedy) → both now say `task deploy` and why; widening the concern corpus stays CC-11.
- INC-3 2026-08-25 The post-commit hook ran a shell `fathom ingest` against the shared store on every commit, which the Index-first rule forbids (`CLAUDE.md`, O16's write path) → HF-2 (`cmd/fathom/postcommithook.go`): the refresh now routes through the daemon's `ingest` MCP tool first, falling back to a store-direct ingest only when the daemon is genuinely unreachable, under IT-15's refusal-on-read-error guard (`TestPostCommitHookPrefersDaemon`, `TestPostCommitHookRefusalIsReported`); status corrected from "pending" 2026-09-23 (C1, DG diagnostics-sweep housekeeping pass) — the fix had already landed and was never flipped.
- INC-2 2026-08-21 A wedged reader wrote a zero-symbol index that reported "0 failed" (O16) → IT-14/IT-15: read failures counted and named, zero-readable runs refuse to overwrite (`TestIngestRefusesEmptyOverwriteOnReadErrors`).
- INC-12 2026-08-25 Law 15 held in the engine, not on the production write path → `init`/`reconcile --apply` write only through `internal/wiring` (`TestInitWritesReceipt`, `TestReconcileStopsOnDriftedRegion`).
- INC-4 2026-08-12 The documented deploy (raw `cp`) took the daemon down fleet-wide (O25) → `.fathom/concerns.cue` deploy_safety; `CLAUDE.md` non-negotiable row; `task deploy` serving assertion.
- INC-5 2026-08-08 Every redeploy re-rolled the TCC grant (O10) → `domain.md` platform fact; `fathom-dev` signing identity in `task deploy`.
- INC-6 2026-07-24 Obligations injection inert for six days after the rename while hooks looked healthy (O15) → deterministic, never-collapsing service resolution (IO-1..3, `cmd/fathom/serviceident_test.go`); doctor `obligations_served` probe.
- INC-7 2026-08-09 "188 sessions" were 8; `Stop` fires per turn (MI-1..3) → `domain.md` platform fact; distinct-session denominators pinned by test.
- INC-8 2026-08-12 The transcript reader hand-copied three of twelve fields, silencing four instruments for weeks (O24) → `TestTranscriptSummaryCarriesEveryField` (reflection over the struct).
- INC-9 2026-07-07 The first gated run caught the plan's author over-claiming → the pre-push `fathom verify` gate itself (`lefthook.yml`).
- INC-10 2026-08-23 Two C++ rows flipped when unrelated CUE tests landed; cause was typed edges silently dropped on a discovery gap → VS-9 `DiscoveryGaps` named on stderr; resolver stability pinned (`docs/plans/verify-spine.md`).
- INC-11 2026-08-21 Push with `LEFTHOOK=0` after the gate ran green but the wedge corrupted transfer → `autonomy.md` "Never" tier names the only allowed form.

**INC-15 (2026-08-27). A design doc cited ADR-027 before ADR-027 existed — the
same mistake as INC-14, by the same author, one day later.** What caught it:
`adr_references_resolve`, the custodian kind built AS INC-14's outcome (EP-14).
It fired within minutes, naming all three citing files, and `fathom doctor`
turned it into a FAIL. What it produced: nothing new to build — the check
already existed and worked. The value is the evidence that a mechanical check
beats an intention: INC-14's remedy was "write the record first", which is
exactly what was resolved to do and exactly what was not done. **Outcome:**
closed by ADR-027 being written; the check stays as the enforcement, and this
entry exists because a repeat under a working check is worth more as evidence
than the first occurrence was.
