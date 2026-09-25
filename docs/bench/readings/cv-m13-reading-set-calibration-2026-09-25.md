# CV-M13: execution evidence calibrated against the reading set (2026-09-25)

**Verdict (plain language).** Static reach is the right first filter and the
wrong last word. On the whole fathom codebase at one head (4,636 non-test Go
functions, 4,631 joined to `go tool cover -func`), reach called 4,534
functions `attested` and 168 of them (3.7%) never actually ran under the
suite — mostly ordinary reach slack (82 of 168 reached by a plain call path
that the specific test run still never took), plus three named, recognizable
mechanisms: a CLI dispatcher switch a given test run doesn't fully walk (39),
interface fan-out that credits a type for a dispatch it never personally took
(31), and an MCP tool's `register*` closure that wires the handler in without
calling it (15, combining two sub-patterns). Reach also missed real coverage
in the other direction: of 91 functions it called `unreached`, 31 executed
anyway, and every one of the 31 got there through a mechanism the first-party
call graph does not trace at all — 16 through the standard `flag` package
calling a flag type's own `String`/`Set`, 8 through `init()`, 2 through a
package-level var initializer (the same untraced timing as `init`, just
written as a value instead of a function), 2 through `errors`-style `Unwrap`,
1 through `encoding/json`'s `MarshalJSON`, 1 through `fmt`'s `Stringer`, and
1 (`procRusageV2`, a darwin resource-usage probe) whose exact mechanism this
reading could not re-confirm against source. On fathom's own two live changed
ranges (32 changed functions total) reach was flawless at function
granularity — every one of the 32 was both `attested` and executed — but
coarse at line granularity: 37 of 269 changed statements across 13 of those
32 functions never ran, and one function, `runServeHTTP`, is the clean single
case that motivated this whole reading: it ran (72.7% of the function
covered), but its three changed statements (`servehttp.go` lines 129-133,
covering line 131, the O119 fix) never did — confirmed twice, once in the
whole-suite merged profile and once in a `servehttp`-focused rerun. The
weaker, less-tested sibling app `conjure` supplied the real negative
examples fathom's own diffs didn't have: of every disagreement its ablation
table produced across three ranges (2 `attested`-but-uncovered items, 5
`unreached` items — a complete hand-check, not a sample), both uncovered
`attested` items turned out to be real false attestations (a func-value
handler registration and an interface-dispatch fan-out, both traced to one
hub test), and of the 5 `unreached` items, 2 were genuinely untested and 3
were false negatives — dispatched through the standard library's
`slog.Handler`, invisible to the graph. **For a reviewer, static reach alone was unreliable in both directions.**
Its `unreached` verdict was wrong for 31 of 91 functions across the whole
fathom head, and for 3 of the 5 items it flagged in conjure. Each of those
functions ran, called by the standard library, the runtime or a package
initializer, where the first-party graph cannot see. Its `attested` verdict
says a passing test's call graph reaches the function, not that the run took
that path: 168 of 4,534 never ran on fathom, and both of the two that never
ran in conjure were false attestations through a hub test. That is why the
reading set now prefers execution evidence (design §3.5; CV-M14 re-measured
it).

Bar this reading measures against (`docs/plans/refactor-conservation.md`
§J, rows CV-67..CV-M14 — see **What it changed** below): CV-M13 calibrates
static-reach attestation against Go coverage on fathom and a weaker-tested
fleet repository; CV-M14 (measured PASS: `docs/bench/readings/cv-m14-execution-evidence-2026-09-25.md`) re-ran these
same ranges once the coverage-based classifier ships and checked that no
`attested` item is unexecuted, no flagged item executed, `runServeHTTP`
reads `partial` at `servehttp.go:131`, the `slog` handler methods read
`attested`, `WithGroup`/`ParseError.Unwrap` read `unexecuted`, and a
`-coverpkg` attest run costs at most twice the plain run.

## Method

Two evidence sources, both against fathom commit
`d41cdf1441d284c96e8517c10bcf581bb14811cb` (the "pushclone HEAD" build,
including 1d7598e1's fixes) as the analysis tool, run from scratch clones —
never the sextant checkout at `/Volumes/m2ext/code/geoffmilleraz/sextant`,
never a shared store:

1. **Fathom on itself**, two ranges chosen to match CV-M12's own dogfood
   ranges: `c75b70e8..4957b916` ("r1") and `5afeee42..78e508d3` ("r2"). Per
   range: `task tests:attest`'s exact shape (`go test -json ./...` piped
   through test-kit's `surveyor go-test-json` adapter, commit-bound) for the
   attestation envelope
   (`method/attest.sh`); a second, separate `go test -json -coverpkg=./...
   -coverprofile=... ./...` run at the *same* head for execution truth
   (`method/cover.sh`); a scratch-store `fathom ingest` +
   `fathom conserve --since <base> --json` for the static reading set
   (`method/readingset.sh`); and a throwaway in-repo test,
   `TestCVM13Dump` (`method/zz_cvm13_dump_test.go`), that re-runs
   `conserve`'s own `Residue` classification loop verbatim but keeps every
   `attested` item too (production `Residue` JSON lists only
   `unattested`/`unreached`) and records, per item, the shortest resolved
   caller path from the changed function to whichever test attested it, with
   the edge kind on every hop. Its totals are cross-checked against a real
   `Residue` call on the same input (both logged in
   `method/logs/readingset-r{1,2}.log`: `changed=1 replica=... real
   att=1 unatt=0 unr=0` for r1, `changed=31 ... att=31 unatt=0 unr=0` for
   r2). `method/join.py` then joins that per-function dump against
   `go tool cover -func` two ways: function-level (`cover_pct > 0.0`) and a
   stricter changed-line level (coverage blocks, merged by maximum count
   across per-package profile sections, that overlap a line
   `git diff -U0 base head` actually added or changed inside the function).
   A second script, `method/wholejoin.py`, repeats the function-level join
   for *every* non-test Go function at the r2 head (not only the 31 changed
   ones), tagging each with path features (`iface-dispatch-hop`,
   `func-value-hop`, `register*-closure-hop`, `mcp-server-construction-hop`,
   `cli-dispatcher-hop`) read off its nearest-attester path — the whole-head
   supplement below.

2. **The `conjure` control**, a separate, less rigorously tested sibling app,
   three ranges (`control/run_range.sh`, one call per range): `r1`
   `87ccb17..e9d0f2e`, `r2` `e9d0f2e..10efbe4`, `r3` `44fd5ae..e9d0f2e` (r3
   reuses r1's store, test run and coverage at their shared head e9d0f2e —
   `control/r3/README-reuse.txt`). Same attest-then-cover-then-conserve
   shape, run with `GOFLAGS=-mod=vendor` (conjure commits `vendor/` and an
   older head's `replace => ../app-kit` doesn't resolve under `-mod=mod`).
   A second throwaway test, `TestCtlDumpClasses` (in a separate `ctl-fathom`
   checkout pinned to the same tool commit, `control/dump_ablate.sh`),
   dumps the baseline class per changed function plus five **edge-kind
   ablations** — the same classification with declared edge kinds excluded
   from the graph — each keyed to a fixed CLI-construction hub,
   `conjure::internal/cli/app.app.newRoot`: `no_iface_dispatch`,
   `no_func_value`, `no_dispatch_no_funcvalue` (both), `no_hub_out_edges`
   (the hub's own out-edges removed), and `no_hub_no_dispatch_no_funcvalue`
   (all three). `control/join_cover.py` joins each range's classes against
   its own `cover -func`; `control/summarize.py` produces the aggregate
   table in `control/summary.txt`. Every hand-checked item (below) was
   independently re-read against its source and its test.

Both evidence sources are throwaway, never-committed measurement scripts
(`method/`, `control/`), not the shipped feature — CV-67..CV-73 (below) are
still unbuilt.

## Fathom ranges: function-level and changed-line-level results

| range | changed fns | conserve residue | function-level | changed-stmt level |
|---|---|---|---|---|
| r1 `c75b70e8..4957b916` | 1 | attested 1 / unattested 0 / unreached 0 | 1/1 attested-executed | 0 changed stmts (comment/signature-only edit) |
| r2 `5afeee42..78e508d3` | 31 | attested 31 / unattested 0 / unreached 0 | 31/31 attested-executed | 232/269 changed stmts executed, 37 unexecuted across 13 fns |

(`fathom/r1-conserve.json`, `fathom/r2-conserve.json`, `fathom/r1-per-function.csv`,
`fathom/r2-per-function.csv`.)

Both ranges' `attest_run` were clean (r1: 6952 pass/0 fail/13 skip of 6965;
r2: 6987 pass/0 fail/13 skip of 7000 — `fathom/run-summary.json`). Every one
of the 32 changed functions across both ranges was both statically
`attested` and, independently, executed at function granularity — zero
disagreements at that grain. The gap only shows up one level down, at the
changed line: 13 of the 31 r2 functions have at least one changed statement
that never ran while the function as a whole did (`fathom/r2-per-function.csv`,
column `unexecuted_changed_blocks`):

| function | file:line | changed stmts | executed | never-run block |
|---|---|---|---|---|
| `runServeHTTP` | `cmd/fathom/servehttp.go:50` | 3 | 0 | `129.73-133.4` (3 stmt) |
| `structFieldShapes` | `internal/analysis/inventory/apicompat.go:116` | 13 | 12 | `118.22-120.3` |
| `interfaceMethodShapes` | `apicompat.go:145` | 17 | 15 | `148.23-150.3`; `166.10-167.12` |
| `typesCompatible` | `apicompat.go:200` | 7 | 5 | `209.98-210.34`; `211.10-212.15` |
| `interfaceCompatible` | `apicompat.go:235` | 10 | 9 | `250.2-250.13` |
| `normalizeAnyType` | `apicompat.go:264` | 22 | 13 | 8 unrun ranges (266-310) |
| `fileLiteralValueKinds` | `apicompat.go:354` | 15 | 13 | `363.11-364.13`; `367.43-368.14` |
| `resolveBasicKind` | `apicompat.go:385` | 13 | 10 | `400.20-401.24`; `404.19-405.23`; `406.19-407.26` |
| `planPathRelativeTo` | `internal/conserve/input.go:297` | 6 | 5 | `302.48-304.3` |
| `Resurrected` | `internal/conserve/resurrection.go:101` | 20 | 18 | `117.17-119.12` |
| `applyIntegrityDimension` | `internal/mcp/review_integrity.go:36` | 27 | 22 | 4 unrun ranges (41-79) |
| `readWorktreeFileForIntegrity` | `review_integrity.go:89` | 7 | 4 | 3 unrun ranges (92-96) |
| `parseIntegrityFileLineCite` | `review_integrity.go:105` | 8 | 5 | 3 unrun ranges (106-116) |

`runServeHTTP` is the clean single-function case: 72.7% of the function
covered (`fathom/r2-cover-func.txt`), but the exact 3 statements it changed
— lines 129-133 — never ran, under either the whole-suite merged profile
(`fathom/r2-coverprofile-maxmerged.txt`) or a second, `servehttp`-scoped
coverage rerun taken independently (`fathom/r2-servehttp-tests-only-coverprofile.txt`,
lines 20-22: `129.9,129.73 1 0` / `129.73,133.4 3 0` / `133.9,135.4 1 0`).
Line 131 sits inside that block. It is the fix from observation O119
(`.fathom/observations.md`): `servehttp.go:131`'s real-run `WithLLM` call now
reads the named rate constants instead of a hardcoded literal — and O119's
own fix note says why coverage shows zero there: the call "doesn't build
without a live LLM provider, which no test in this suite exercises." Two
independent sources (a coverage profile and a human bug-fix writeup) agree
on the same untested line.

## Whole-head supplement: every function, not only the changed ones

`fathom/wholejoin.py` classed and joined all 4,636 non-test Go functions at
the r2 head (4,631 joined to `cover -func`, 5 unjoined —
`fathom/r2-wholehead-per-function.csv`/`.json`). Class counts among the
joined 4,631: `attested` 4,534 (4,366 executed / 168 never), `unreached` 91
(31 executed / 60 never), `unattested` 6 (3 executed / 3 never — `unattested`
is also the class of all 5 unjoined rows, so 11 `unattested` functions exist
in total at this head; this reading does not otherwise examine them, since
`unattested` means no reaching test at all — there is no execution question
to calibrate).

**4,534 attested, 168 (3.7%) never executed**, by the nearest-attester
path's edge-kind features:

| category | count |
|---|---|
| plain direct path (ordinary reach slack — the test's own run just didn't take that branch) | 82 |
| CLI-dispatcher hop (`.run`/`.runWithStdin` switch not fully walked by the attesting test) | 39 |
| interface-dispatch hop (credited by dispatch to an interface method, not a call to this concrete implementation) | 31 |
| interface-dispatch + `register*`-closure + MCP-server-construction hops | 13 |
| `register*`-closure + MCP-server-construction hops (tool registered, handler never invoked) | 2 |
| func-value hop (passed as a value, never called) | 1 |
| **total** | **168** |

**91 unreached, 31 executed anyway**, all zero-hop (no first-party path at
all) — every one reached through a mechanism outside the traced graph:

| category | count | examples |
|---|---|---|
| `flag.Value` (`String`/`Set` pairs dispatched by the stdlib `flag` package during CLI flag parsing) | 16 | `stringList`, `repeatedFlag`, `stringSliceFlag`, `optionalStringFlag`, `profileFlagValues`, `planFlags`, `armCloneFlag`, `armSetupFlag` (8 types × 2 methods) |
| package `init()` (runtime-invoked at package load, no call edge) | 8 | `internal/conserve/{certificates,deprecatedcallers,governance,plane_integrity,resurrection,surface}.go`, `internal/extractor/tsextract.go`, `internal/mcp/meta.go` |
| package-level var initializer (same init-time execution as `init()`, called from a `var` literal instead — confirmed by reading `internal/harness/descriptors.go`: `pullTierVehicles()` is called inside `var agents = Descriptor{... Vehicles: pullTierVehicles() ...}`; `withKey` is called only from inside other package-level `Descriptor` literals) | 2 | `pullTierVehicles`, `withKey` (`internal/harness/descriptors.go`) |
| `error.Unwrap` (dispatched by `errors.Is`/`errors.As`) | 2 | `LoadError.Unwrap`, `claimRejectionError.Unwrap` |
| `json.Marshaler` (dispatched by `encoding/json.Marshal`) | 1 | `summaryEnvelope.MarshalJSON` |
| `fmt.Stringer` (dispatched by a `%v`/`%s` format verb) | 1 | `ConformanceLevel.String` |
| unconfirmed | 1 | `procRusageV2` (`cmd/fathom/rss_darwin.go:42`, darwin resource-usage probe, 75.0% covered, 0-hop) — the exact mechanism was not independently re-verified: its head commit is unreachable in the current checkout, and the file no longer exists under that name |
| **total** | **31** | |

## The conjure control

| range | raw changed | classified | attested | unreached | unindexed (vendor) |
|---|---|---|---|---|---|
| r1 `87ccb17..e9d0f2e` | 54 | 54 | 54 | 0 | 0 |
| r2 `e9d0f2e..10efbe4` | 326 | 226 | 225 | 1 | 100 |
| r3 `44fd5ae..e9d0f2e` | 148 | 148 | 144 | 4 | 0 |

(`control/summary.txt`, `control/r{1,2,3}/join.out` first line.)

**The vendor/ count mismatch.** `control/summary.txt`'s r2 `baseline` line
reads `{'attested': 225, 'unreached': 1, 'unindexed(unreached-or-unanalyzed)':
100}` — 100 of the 326 raw changed functions in `classes.json`'s own
`functions` list carry that fourth, non-residue class. All 100 are, every
single one, under
`vendor/github.com/geoffmilleraz/app-kit/appkit/ui/...` (`control/r2/join.out`,
`cov=None` for each — confirmed by grep, zero non-vendor unindexed rows). A
vendored app-kit UI bump landed inside the same diff and git counts it as
100 changed functions, but fathom's static reading set (`residue_counts`:
`{'attested': 225, 'changed': 326, 'unattested': 0, 'unreached': 1}`) only
classifies 226 of the 326 — the other 100 sit outside `attested`/
`unattested`/`unreached` entirely, visible only in the raw `functions` dump,
not in `residue_counts` itself. The real classified denominator for r2 is
226, not 326.

**The test-case join gap: 34/566 (r1) and 41/1084 (r2).**
`control/r1/tests-ingest.out`: "566 case(s) (532 joined)"; `r2`: "1084
case(s) (1043 joined)" — matching `store-summary.txt`'s `test_cases join`
rows. `fathom tests ingest` joins a case to a `Test*` symbol by bare name;
a name shared across packages joins to neither
(`docs/design/refactor-conservation.md` §3.5, "Test identity"). Each
range's `store-summary.txt` lists the unjoined *top-level* case names (its
own query excludes subtest names, which contain `/`): r1 names 9 —
`internal/provider/{brewprov,defaultsprov}.{TestDomain,TestObserve,TestPlan,TestVerify}`
plus `fileprov.TestDomain`; r2 names 11, the same 9 plus
`internal/adopt.TestHelpers` and `internal/cli.TestHelpers`. Both lists are
shorter than the raw gap (9 vs 34, 11 vs 41), so most of the gap is likely
each ambiguous top-level test's own subtests inheriting the unresolved
join — this reading did not re-query the (now-deleted) scratch store to
confirm that inference directly.

**Hub-test attribution.** In every range, one end-to-end CLI test attests
nearly everything: `conjure::internal/cli/cli.TestAdoptBrew` is the
`attesting_test` for 53 of 54 attested functions in r1, 191 of 225 in r2,
and 143 of 144 in r3 (`control/r{1,2,3}/classes-with-coverage.tsv`). r2's
other 8 attesters carry single digits each (`TestScannerMetadataCannotIntroduceDeclarations`
9, `TestAdoptionRejectsChangesToExistingFacts` 9,
`TestBootstrapContractGolden` 7, `TestAdoptionRollbackCleansNewDirectories`
5, four more at 1). The test named as voucher was, in this control, always
this one hub.

**The two attested-but-never-executed items and their hand check**
(`control/handcheck.tsv`):

- `jsonShapeError.Error` (`internal/provider/brewprov/classify.go:68`,
  added, commit `9bcf018`, in r1 and r3) — 0.0% covered despite 139
  reaching tests and 40 resolved callers. Witness path:
  `canon_test.TestParseErrors` dispatches `.Error()` on every registered
  canon error type through an interface-dispatch edge — it exercises the
  interface method, never this type's own construction. **FALSE
  ATTESTATION (confirmed never executed).** Removing interface-dispatch
  edges from the graph (`no_iface_dispatch` ablation, table below)
  reclassifies exactly this one item to `unreached` in both r1 (54→53
  attested) and r3 (144→143 attested) — the ablation isolates the same
  root cause the hand check found.
- `webui.handleAdoptStream` (`internal/webui/adopt.go:263`, modified,
  commit `c996187`, r2) — 0.0% covered despite 163 reaching tests, only 1
  resolved caller. Witness path: `TestAdoptBrew` → `runCLI` → `runCLIAt` →
  `app.run` → `newRoot` → `newUICmd` → `webui.New`, a func-value hop that
  registers the handler as a route — the hub test builds the router, never
  issues the `GET /stream/adopt` request that would call it. **FALSE
  ATTESTATION (confirmed never executed).**

**The five unreached items and their hand check:**

- `ParseError.Unwrap` (`internal/uiprefs/store.go:36`, r2, commit
  `c996187`) — 0 reaching tests, 0.0% covered. `store_test` uses
  `errors.As` (matches the top-level `ParseError`, never unwraps) and its
  `errors.Is(os.ErrNotExist)` case is never a `ParseError`. **TRUE POSITIVE
  (genuinely untested).**
- `fanoutHandler.Enabled` / `.Handle` / `.WithAttrs`
  (`internal/logx/logx.go:181/190/203`, r3, commit `5958dcb`) — each
  100.0% covered despite 0 resolved callers. `logx_test`'s
  `slog.Default().Info`/`.Debug`/`logx.Setup(...).With(...)` calls dispatch
  through the standard library's `slog.Logger` to the `fanoutHandler` as a
  registered `slog.Handler` — real execution, no first-party edge to see
  it. **FALSE FLAG × 3 (confirmed executed; the graph's blind spot is the
  same stdlib-interface-dispatch pattern as the whole-head supplement's 31
  cases.)**
- `fanoutHandler.WithGroup` (`logx.go:211`, r3, commit `5958dcb`) — 0.0%
  covered, no call anywhere in first-party code or tests. **TRUE POSITIVE
  (genuinely untested).**

Net: 2 of 2 hand-checked `attested`-but-uncovered items were real tool
errors; 2 of 5 hand-checked `unreached` items were real (untested code
correctly flagged), 3 of 5 were false negatives from one specific,
nameable cause (stdlib interface dispatch). This was an exhaustive
hand-check of every disagreement cell the ablation table below produced
across all three ranges, not a sample.

**Edge-kind ablations** (`control/summary.txt`; `attested`/`unreached`
counts per variant):

| variant | r1 (base 54/0) | r2 (base 225/1) | r3 (base 144/4) |
|---|---|---|---|
| `no_iface_dispatch` | 53/1 | 224/2 | 143/5 |
| `no_func_value` | 53/1 | 171/55 | 143/5 |
| `no_dispatch_no_funcvalue` | 52/2 | 170/56 | 142/6 |
| `no_hub_out_edges` | 41/13 | 199/27 | 103/45 |
| `no_hub_no_dispatch_no_funcvalue` | 39/15 | 144/82 | 101/47 |

Removing only interface-dispatch edges barely moves attested counts in any
range (54→53, 225→224, 144→143 — one item each time). In r1 and r3, that
one item is exactly the hand-confirmed `jsonShapeError.Error` false
attestation (both ranges share its commit, `9bcf018`); r2's one-item drop
is a different function this reading did not separately hand-check.
Removing the hub's (`app.newRoot`'s) own out-edges costs far more in every
range (54→41, 225→199, 144→103) than either dispatch kind alone, and
combining all three exclusions collapses attested counts furthest (54→39,
225→144, 144→101). Static reach in this control is carried overwhelmingly
by one hub's construction-time wiring, not by interface fan-out — which
also explains the hub-test-attribution finding above: the hub function and
the hub test are, structurally, the same story.

## Caveats

- **A merged profile cannot attribute execution to a test.** Every
  execution-truth number above comes from one whole-suite `-coverpkg`
  profile per range (`go test -coverpkg=./... ./...`), never a per-test
  profile. "Executed" means the function ran somewhere in the suite; it
  answers nothing about *which* test drove it. The static reading set's own
  `attesting_tests`/`first_attester` columns remain the only "which test"
  answer this reading has, and they are static-reach claims, not execution
  claims (design §3.5: "tests stays the static reach, never 'the test that
  executed it'").
- **In-process only.** `go test -coverprofile` instruments only the test
  binary's own process. A path that only runs through a spawned
  subprocess, a separate `exec.Command`, or an external tool invocation
  would read as never-executed here even if genuinely exercised elsewhere.
  This reading did not separately verify that none of the items above
  cross a process boundary.
- **A weak control: few genuinely untested functions to catch.** Across
  all three conjure ranges only 5 changed functions came back `unreached`
  at all (0 in r1, 1 in r2, 4 in r3), against 54+225+144=423 `attested`.
  The 3-of-5 false-negative rate found by hand-checking those 5 is a
  small-sample result, not a broad-sample estimate of how often stdlib
  dispatch fools the reading set — a repo with more real dead code would
  give a firmer number.
- **A timing-gate failure under load.** CV-M14's own bar caps `-coverpkg`
  overhead at 2× the plain run before the profile becomes opt-in. Measured
  wall-clock here: r1's coverage run (179s, `method/logs/cover-r1.log`)
  over its attest run (134s, `method/logs/attest-r1.log`) is 1.34×; r2's
  coverage run (122s) was *faster* than its attest run (127s) —
  neither range is close to tripping a 2× circuit breaker on aggregate
  wall time. But r1's coverage-instrumented run separately failed a real,
  unrelated wall-clock-budgeted unit test,
  `internal/fusion.TestTypedEdges_PerformanceGate` (`fathom/run-summary.json`:
  `coverage_run.failed`), which passed cleanly in the plain attest run on
  the same commit; r2's coverage run had zero failures. Coverage
  instrumentation cost one collateral failure in a performance-gated test
  on one of the two ranges without ever tripping the aggregate-time
  threshold meant to catch exactly this class of risk — a gap CV-M14's
  literal bar (aggregate suite wall time only) does not yet cover.

## What it changed

`docs/design/refactor-conservation.md` §3.5 "Execution evidence (CV-M13)"
(uncommitted at measurement time, in the working checkout this reading
was taken from — the sextant checkout itself was never modified by this
reading) already names this document and cites its headline numbers (168
of 4,534, 3.7%; 31 of 91; `runServeHTTP`/O119) ahead of this reading being
written; this document is the measurement that backs those citations.
Plan rows **CV-67..CV-73** (`docs/plans/refactor-conservation.md` §J)
specify the still-unbuilt production feature this calibration argues for:
capturing a commit-bound coverage profile from the same `tests:attest`
run (CV-67), mapping its blocks to files and merging duplicates by maximum
count (CV-68), classifying each changed function `attested`/`partial`/
`unexecuted` by changed-block execution when the profile is usable
(CV-69), falling back to static reach with a named reason when it isn't
(CV-70), reconciling `changed` against classified-plus-`unanalyzed`
(CV-71), joining test cases to symbols by package when a bare name is
ambiguous (CV-72), and Review Kit's `refactoring.uncovered-behaviour`
gate reading the new classes (CV-73, cross-repo, skipped here). None of
CV-67..73 is implemented; this reading measured the *current*
static-reach-only classifier against coverage taken as a separate,
throwaway join, not a shipped feature. **CV-M13** itself (the plan row
this document satisfies) is an "op" row pointing at this file. **CV-M14**
remains open: it requires the real coverage-based classifier to exist and
be re-run on these same ranges, predicting `runServeHTTP` reads `partial`
at `servehttp.go:131`, the `slog` handler methods read `attested`,
`WithGroup`/`ParseError.Unwrap` read `unexecuted`, and the `-coverpkg`
attest run costs at most 2× the plain run. Every number in this reading is
consistent with those predictions (the changed block containing line 131
never ran; the `slog` handler methods were 100% covered; `WithGroup` and
`ParseError.Unwrap` were 0% covered; wall-clock stayed under 2× on both
ranges) — but CV-M14 can only be checked off once CV-67..69 actually ship
and produce those literal class labels themselves; this reading used a
throwaway test-only script, not the production path.

## Reproduce

All scripts and raw output are preserved under
`/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/cv-m13/`
(fathom side: `fathom/method/{attest,cover,readingset}.sh`,
`fathom/method/{join,wholejoin}.py`,
`fathom/method/zz_cvm13_{dump,whole}_test.go`; conjure control side:
`control/run_range.sh`, `control/dump_ablate.sh`, `control/join_cover.py`,
`control/summarize.py`). Per fathom range: `readingset.sh <clone> <base>
<tag>` (static reading set + throwaway dump), `cover.sh <clone> <tag>`
(execution truth), `attest.sh <clone>` (the `tests:attest` envelope), then
`join.py <tag> <base> <head>` / `wholejoin.py` to produce the per-function
and whole-head CSV/JSON this reading cites. Per conjure range:
`run_range.sh <tag> <base> <head>` end to end, then `dump_ablate.sh <tag>
<base> <head> <storeTag>` per ablation variant, then `join_cover.py` and
`summarize.py`. All environment variables (`GOCACHE`, `GOMODCACHE`,
`GOTMPDIR`, `TMPDIR`, `GOWORK=off`, `CGO_ENABLED=1`,
`MACOSX_DEPLOYMENT_TARGET=14.0`, and `GOFLAGS=-mod=vendor` for conjure
only) are set at the top of each script. On `go: ... operation not
permitted` from the external volume's TCC wedge: `tccutil reset
SystemPolicyRemovableVolumes com.mitchellh.ghostty`, then retry the exact
same command (this reading needed it once, mid-session, for an unrelated
source read — none of the measurement runs themselves hit it, per their
clean exit logs).
