# Downstream integration evidence

Date: 2026-10-21. Candidate code: 5e939468d5775d32207dcc80a8026c844c409d98.
Fathom source: 07b249ca5d7811f9778f3ec111f59e6d029ee0bc.
The final declared `task downstream-check` **passed**, exit 0 in 664 seconds.
The final publication change after this tested code revision contains only
evidence and tracking updates; no runtime, schema, CLI or gate code changed.

## Scope and prepared snapshots

Tests use disposable local checkouts. The actual source checkouts for consumer-a,
consumer-b and consumer-c retain their pre-existing dirty work. Fathom's
clean canonical checkout was intentionally advanced to its published repairs and
updated through its sanctioned `task deploy`; the daemon asserted revision
07b249ca5d78. No consumer dependency upgrade is claimed from scratch-only changes.

| Consumer snapshot | Preparation and result |
|---|---|
| Fathom 07b249ca5d7811f9778f3ec111f59e6d029ee0bc | Clean committed source. Exact candidate mirror sync, inverse vendor verification, CLI and all-package builds, owned race tests and separate performance guard: passed. All 2,234 command tests passed; the unchanged performance target passed in 31.255 seconds. |
| consumer-a a007075fab6ac156be44de46df941cce4a0915ed | Scratch-only uniseg v0.4.7 materialization, checksum from its existing sqlite v1.52.0 graph and vendor synchronization. Final aggregate build and tests: passed. |
| consumer-b 8b8620936afd2ebcff6810baec1ef37528699c1a | Scratch-only uniseg materialization and exact ignored financials/students/marketing fixtures. Private inputs remain local and uncommitted. Final aggregate build and tests: passed. |
| consumer-c 04be1238bae18e9542b135060233412b4abd72ca | Scratch-only uniseg materialization; the preserved consumer-c task re-pins and re-vendors exact candidate v0.9.1-0.20261021111612-5e939468d577. Build, lint and vendor guard: passed. |

The three non-Fathom consumers initially refused before build on their committed
dependency manifests. Prepared passes establish the stated snapshots, not upgrades
to those original workspaces. The separate app-kit reverse-check is unconfigured
in disposable Fathom clones and explicitly skips. Its prior pinned reverse-check
was exercised separately; it is not a fifth final aggregate consumer.

## Corrected Fathom contract

ADR-070 rewrites kit imports into Fathom's in-module `third_party/agentic-kit`.
A sibling go.work replacement cannot substitute a candidate for those imports.
Copied mirrors intentionally omit or rename upstream module metadata and assets;
raw `go test ./...` also runs unsupported upstream tests. Fake module files,
replacement goldens and assertion skips were rejected.

The approved `scripts/fathom-check.sh` clones clean committed sources outside both
modules, syncs the exact candidate through Fathom's own vendor script, asserts the
resulting pin, and runs vendor verification, the CLI build, all-package compilation,
the owned race suite and the separately declared performance target. GOWORK is off.
Only race tests receive `FATHOM_TEST_GOMAXPROCS` (default 2); build and performance
retain caller settings. The final run leaves caller GOMAXPROCS unset.

Lifecycle tests prove command order, exact pins, capacity scoping/override/refusal,
Git-environment isolation, failure propagation, dirty-kit refusal, cleanup and
source preservation. Tests also run with inherited Git variables. Missing consumers
remain explicitly untested. The owner authorized this gate repair on 2026-10-21.

## Failures retained and resolved

| Trial | Observed result and disposition |
|---|---|
| Original raw Fathom command, kit 1ee09f4 / Fathom 8d11ad68 | Build passed; copied upstream tests failed on absent metadata/goldens, workspace-sensitive fixture behavior and a query-time budget. It neither substituted nor validated the candidate mirror. Replaced by the actual consumer-owned workflow above. |
| Initial mirror pilot, kit 1ee09f4 / Fathom 8d11ad68 | Vendor, build, owned race tests and separate performance gate passed in a prepared warm snapshot. An ignored inherited go.work was removed only from scratch. This earlier pass did not prove later candidates. |
| Default helper, kit 9f8349e / Fathom 8d11ad68 | One preflight test lost its deadline diagnostic/beat during Git discovery. A fourth-call-only 150 ms delay reproduced the early return; original Git latency was not measured. Fixed upstream in 2974c3153; O10 records the causal boundary. |
| Global GOMAXPROCS=2, kit 9f8349e | Owned race suite passed; CT-5 failed at 11.50x (159.084747 / 13.837751 CPU seconds). The same retained clone passed standalone performance at default concurrency. Scope the cap to correctness tests; never relax the guard. |
| Scoped cap, kit afb436d9 / old Fathom | Owned race suite passed; fresh CT-5 failed at 16.90x. Fresh-cache diagnostics after CLI build and CLI plus all-package build also failed at 19.56x and 16.08x. Cache trimming alone was rejected as the explanation. |
| Matched scope, kit 5e939468 / Fathom ea9cdc7d | Vendor, both builds and 2,234 command tests passed; final aggregate stopped at CT-5: typed 164.430820 / syntax 13.328030 CPU seconds = 12.34x. Same discovered population: 6,496 files, 3,829 Go files, 464 packages, zero typed load failures, 188,998 in-scope edges. No downstream pass was claimed. |
| Source-loading repair, kit 5e939468 / Fathom 07b249ca | Final declared aggregate passed, exit 0 in 664 seconds, including all four prepared consumer targets. |

Two upstream loading defects explained the remaining failures. Syntax honored
connector exclusions while typed loading selected unrelated package roots; both
now use the same selected scope. Then pinned x/tools v0.45.0 requested expensive
export compilation before rechecking selected source. Supported `NeedDeps` source
loading preserves full AST/type information and avoids that duplicated compiler
work. Dependency errors now remain visible before cache reuse, including invalid
imported bodies. Failed packages retain last-good rows and are reported failed,
not silently cached. No dependency version, performance threshold or test assertion
was weakened. Body elision was rejected because it created false import errors.

See Fathom's [source assessment](https://github.com/GeoffMillerAZ/fathom/blob/07b249ca5d7811f9778f3ec111f59e6d029ee0bc/docs/assessments/2026-10-21-typed-scope.md)
for red-first tests, profiles and distinct warm/fresh-cache readings. The 1.70x
source-loading profile was diagnostic, not the final candidate gate. Successful
normal CT-5 output suppresses its passing ratio; no extra benchmark is needed to
claim a guard pass.

## Upstream proof and resource boundary

Fathom 07b249ca passed full validation, normal pre-push validation and exact-tree
proof: 2,273 claimed rows, 2,273 verified, zero failing. Its unchanged push canary
passed with maximum RSS 3,434,938,368 bytes (3,275 MiB). Physical footprint peaked
at 4,304,899,984 bytes, above 4 GiB. Source loading retains more dependency data;
this is not a portable memory-fit or production-latency promise. Optional review
in that isolated clone configured only 3/8 dimensions; exact-tree proof and
independent source review are separate evidence.

Local logs and receipts remain in
`/Volumes/m2ext/tmp/agentic-kit-downstream-20261021-q2x9LG`, including the failed
9f8349e/global-cap and 5e93946-ea9cdc7 runs and the final
`downstream-check-5e93946-07b249c.log`. Upstream publication/deployment logs remain
at `/Volumes/m2ext/tmp/fathom-source-deps-push.log` and
`/Volumes/m2ext/tmp/fathom-source-deps-deploy.log`. These machine-local diagnostics
are not portable artifacts. This durable summary records their scope and outcome.
