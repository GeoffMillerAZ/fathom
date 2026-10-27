# The adopter's game-development patch series against main (2026-10-26)

The series on branch `docs/game-development-support-20261022` (`docs/contributions/game-development-support/patches/`)
was triaged patch by patch against main at ef22c88fe: whether it applies, whether main already does it, which GDEV row
or observation it serves, and whether it is an ENGINE change. Seven read-only critics did the triage, each in its own
scratch worktree, and an eighth wrote the synthesis below. This record is NP-W3's starting point in
the next-phase plan (`2026-10-26-next-phase-plan.md` in this directory, landing separately).

Controller rulings on the synthesis's questions:
- **Q1:** GDEV-05 closes on citation freshness, which main already chose over the source-window server, so 0008 is
  superseded.
- **Q2:** 0025 adds one log line, so the table freeze does not hold it. It lands after the full gate once that is
  confirmed in its diff.
- **Q3 and Q4:** decided when W8 and the 0001 slices are built.
- **Q5:** the .NET 10 host and the licensed Unity run are the owner's to supply.

## Adopter patch series: landing plan

Verdict: of 31 inputs, 20 are already on main, 2 are refused, 8 are adapted in slices, and 1 lands unchanged once a hold lifts. One carry is urgent: a live egress defect in the CUE typed pass.

### 1. Triage

| Patch | Subject | Rec | Serves | ENGINE | Eff |
|---|---|---|---|---|---|
| main-0001 | current-source + native evidence | adapt, decompose | GDEV-02,05,10,11,14,21,25,27,28,30 | yes | L |
| 0002 | window budget | superseded | GDEV-05 | no | S |
| 0003 | window changelog | superseded | GDEV-05 | no | S |
| 0004 | parser admission | superseded | none | no | S |
| 0005 | plan graph on demand | superseded | GDEV-02,25 | yes | S |
| 0006 | preflight expiry | superseded | none | no | S |
| 0007 | native payload sharing | adapt, held | GDEV-30 | no (needs schema) | S |
| 0008 | window report fit | adapt, conditional | GDEV-05 | yes | M |
| 0009 | context nav deps | superseded + note fix | GDEV-02 | yes | S |
| 0010 | validation buffers | adapt, fold | GDEV-05,30,02 | yes | S |
| 0011 | receipt payload release | adapt, squash | GDEV-05,02 | yes | S |
| 0012 | receipt mutex copies | adapt, squash | GDEV-05 | yes | S |
| 0013 | fixture isolation | superseded | none | no | S |
| 0014 | bounded review reads | superseded | GDEV-03,05 | yes | S |
| 0015 | review graph reuse | adapt, slice | GDEV-03,02 | yes | M |
| 0016 | governed loader | superseded | GDEV-02,03 | yes | S |
| 0017 | dimension status | superseded | GDEV-04 | yes | S |
| 0018 | unused import | superseded | GDEV-04 | no | S |
| 0019 | v3 Taskfiles | superseded | GDEV-20 | yes | S |
| 0020 | conserve streaming | superseded | GDEV-03 | no | S |
| 0021 | Taskfile test | superseded | GDEV-20 | yes | S |
| 0022 | generated header | superseded | GDEV-03 | no | S |
| 0023 | skip surface reads | superseded | GDEV-03 | no | S |
| 0024 | ambiguity index | superseded | GDEV-02 | yes | S |
| 0025 | pre-shed memo stats | land, held | GDEV-01 | no | S |
| 0026 | projection capacity | superseded | GDEV-02 | no | S |
| 0027 | reopen CX-18 | refuse | none | no | S |
| O23-0001 | review envelope cap | superseded | GDEV-04 | yes | S |
| O23-0002 | RawMessage selector | superseded | GDEV-04 | no | S |
| R1 | Roslyn declarations | adapt | GDEV-10,11,21 | no | S |
| P1..P3 | Python 3.13 fork | refuse | GDEV-12 | no | S |

### 2. Landing order (one ENGINE change per wave)

1. **W1 ENGINE**: the CUE registry refusal from 0001 (`cueregistry.go` plus its contract test). Also fix the false "Hermetic" comment at `internal/fusion/typedcue.go:183-191`. GDEV-14.
2. **W2 docs/bench**: R1 with its four fixes (cite `tsSignature`, keep `obj/` under `workspace/`, document restore, attach a .NET 10 `verify.py` receipt). Rebind GDEV-04 to `capReviewDiffEnvelope`. Correct the stale ledger rows for 0002, 0005 and 0009.
3. **W3 ENGINE**: 0015's reuse slice. Pass `ImpactGraph` and `snap.Nodes` to the test-gap, resolution, layers and receipt dimensions. This saves two graph builds per cold review.
4. **W4 ENGINE**: 0009 follow-up. Correct the cold-note text for evicted typed edges and keep the prefix.
5. **W5 ENGINE**: callgraph suffix demand and adjacency capacity (0001), with before/after cold-build figures.
6. **W6 ENGINE**: `readFetchSized` and `ExtractRefsBytes` (0001), which allocate on the ingest read path.
7. **W7 ENGINE**: the scip-typescript object-literal fix (0001).
8. **W8 ENGINE**: C# attribute-only test candidates (0001). This is Breaking and needs a migration note.
9. **W9 ENGINE**: SCIP source receipts (0001), with the 0010 SCIP half and 0011+0012 as one commit.

Held until RT-M1 reads PASS:
- **H1**: 0025 unchanged, after the full gate.
- **H2 ENGINE**: the native NUnit columns, with 0007 and the 0010 Unity half.
- **H3 ENGINE**: serve-http store close (0001).

### 3. Coverage

Every P0 row needs GDEV-01's cold-arm receipt, so the series alone closes no row.

- **GDEV-01**: unbuilt (operational). H1 adds pre-shed estimates.
- **GDEV-02**: largely on main, plus W5/W6. Cancellation and named admission are unproven.
- **GDEV-03**: on main, plus W3. Waits on receipts.
- **GDEV-04**: built. Verifiable after W2's rebind.
- **GDEV-05**: citation freshness on main, receipts in W9. Exact windows depend on Q1.
- **GDEV-06/07/08**: unbuilt and untouched.
- **Later rows**: GDEV-14's egress is fixed (W1), but its verdict is unbuilt. GDEV-20 is landed. GDEV-10/11 are partial. GDEV-30 is held. GDEV-40 is unbuilt.

**O1014 stays open.** W5 lowers build cost, but graph residency at shed is RT work. **O1446 stays open.** W3 trims review_diff. Still unbuilt: cold partials for review_diff's non-graph sections and the eviction half.

### 4. Refusals and supersessions

**Refused**
- **0027**: its premise is false on main. `trace.go:243` calls `traceRequirements` from product code.
- **P1..P3**: fathom would own a producer fork, and the `_handleMemoryHighUsage` no-op breaks ADR-047. The `json.py` finding goes upstream.
- **0001 slices**:
  - the `<CONSUMER_SERVICE>` default breaks ADR-011.
  - the window server is superseded by citation freshness.
  - `traceRequirementsWithDecisions` would reopen CX-18.
- **0015 slices**:
  - the global memo fence would fail every service's `GraphFor` during any patch, which worsens O1446.
  - the hard refusals contradict main's best-effort tests.
- **0009 prefix**: it is a documented contract.

**Superseded**
- **Same change on main**: 0004 (75b083c33), 0005/0024/0026 (f3b125206), 0006 (2974c3153), 0013 (688224ea1), 0019/0021 (cddfdc6a7), 0020, 0022 (f073c1180), 0023 (8ab516c2e).
- **0014** (43e860685): `obligationGaps` was dropped deliberately.
- **0017/0018/O23-0001/O23-0002** (6e3276924, 8683baace): always-on status would break byte parity.
- **0016**: it repairs only 0015's bundle.
- **0002/0003**: their host, the window server, is not on main.

### 5. Owner questions

- **Q1.** Should GDEV-05 ("exact source windows") close on citation freshness? If yes, reclassify 0008 as superseded. Its "adapt" conflicts with the 0001 window-server refusal.
- **Q2.** Lift the RT-M1 hold for 0025 alone? It adds one log line and gives O1014 the shed-eviction evidence.
- **Q3.** Accept W8's Breaking C# rule in the next release?
- **Q4.** Several 0001 portions have no disposition: the assetrefs CLI, the Roslyn oracles (do they overlap R1?) and the `ingest_generation` refs. Carry, hold or refuse each?
- **Q5.** Who supplies the .NET 10 host for R1 and the licensed Unity run for GDEV-10/11?
