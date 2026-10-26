# Catalog workflow integration evidence

Date: 2026-10-21. Baseline main: 5d4c989ce18df24f91f479b7e11ab1e2b3b386de.
Imported contribution: 89fbe33acc4efd6d29de34357f6353cb13d3ca08.
Specification: [catalog-workflow](../../../designs/catalog-workflow.md).
Contribution dispositions: [inventory](contributions.md).

| Instrument | Result and scope |
|---|---|
| Full repository gate | `task validate` passed after the final review fixes (exit 0), including fmt, vet, all CUE fixtures, constitution lint, shell checks and root/nested race suites. |
| Focused race tests | `go test -race ./kits/catalog-kit ./cmd/catalogctl ./cmd/internal/catalog ./foundations/knowledge/catalog` passed. Later focused catalog/CLI reruns include definition, empty collection, pending tree and output failure regressions. |
| Library/CLI review | Independent review identified missing definition identity, profile selection, packet bounds and output classification. Fixed, retested; final follow-up reports no blocker. Contextual checks intentionally apply to selected composition, structural checks to all loaded layers. |
| Review Kit pilot | Passed actual LoadStandardWithSources/FlattenCatalogs/BuildStandard, metadata identity reconciliation, retained typed definition/source-span/hash/origin sidecars and semantic-floor refusals under race tests. |
| Env Kit pilot | Passed actual LoadWith projection, exact typed-entry coverage, additive/refinement rows and typed contradiction refusal under race tests. |
| Pilot revision | platform-repo 821c509035a8bbd98907c83b54372bec70116425. Replay: `bash kits/catalog-kit/pilots/run.sh`. Isolated archived copy, no consumer checkout changes. |
| Parquet | Python 3.12 + PyArrow 20.0.0 in an isolated uv environment: two test methods passed, nonempty/empty roundtrips and prior-output preservation on refusal. Root Go dependencies unchanged. |
| Skills | All four catalog skills passed skill-creator structural validation; changed declare/doctor skills revalidated. Original read-only doctor trial retained. A new routing trial selected catalog-doctor and correctly diagnosed a changed definition at version 1: exit 1, empty stdout, definition digest mismatch; no auto-grant or input edits. |
| Fathom | All ten code rows pass after managed service recovery. CK-5 is reached through the actual CLI identity/key path, with no library/test waiver. Checked-plan `--strict --require-all` passes: 10 claimed, 10 verified, 0 failing. Runtime tests separately passed; static reachability alone is not runtime proof. |
| Downstream | Final declared aggregate passed (exit 0, 664 s) at clean committed kit 5e939468 and Fathom 07b249ca: exact mirror/vendor verification, CLI and all-package builds, owned race tests (2,234 command tests), unchanged performance target, consumer-a/consumer-b build and tests, and consumer-c re-pin/vendor/build/lint guards. Final publication delta is evidence/tracking only. These are prepared snapshots, not production adoption; app-kit reverse-check was explicitly unconfigured in the helper clone. See [downstream evidence](downstream.md). |
| Actual CLI smoke | Resolved CUE-exported spec/policy/profile; doctor checked 2 layers and 3 nodes. Prepared exact-definition/evidence request plus reuse key without a provider. scan-go reconciled 210 packages (248 rows including groups). |
| Fathom wiring | Reconciled by the installed generator with receipt 6a9d12c5dfbe. The local ownership/receipt files are ignored machine state. |
| Upstream Fathom repairs | Deadline reporting fixed in 2974c3153; selected package roots and matched CT-5 scope in ea9cdc7d5; source dependency loading and transitive error disclosure in 07b249ca5. The final tree passed full validation, normal pre-push validation, exact-tree proof (2,273 claimed rows) and the unchanged RSS canary. Maximum RSS was 3,434,938,368 bytes; physical footprint was 4,304,899,984 bytes, above 4 GiB, so this is not a universal 4 GiB fit claim. Commits are on Fathom main; sanctioned `task deploy` asserted the managed daemon serves 07b249ca5d78. See [upstream assessment](https://github.com/GeoffMillerAZ/fathom/blob/07b249ca5d7811f9778f3ec111f59e6d029ee0bc/docs/assessments/2026-10-21-typed-scope.md). The clone's optional review had 3/8 dimensions configured, separate from exact-tree proof and independent source review. |
| Measured DX gap | At 7373c9b, integration-branch pre-push passed in 355.43 s: Fathom pushed-tree proof 320.97 s, repository validation 30.94 s. Preserve source-bound proof; investigate keyed reuse/phase timings separately before changing this gate. Explicit deep reviewer markers were also denied as worker routing in this host; installed binary/policy match source, runtime envelope cause remains unknown. Standard-tier independent reviews completed. |
| Provider/live evaluation | No provider calls, paid runs or model-quality claims. |

Implementation and bounded integration do not establish production migration,
measured human setup time, saved operator effort, calibration or universal catalog
semantics. Domain payloads and authority remain with each consumer. The optional
Parquet environment does not alter go.mod. Production migration and future protocols
have explicit dispositions/triggers in the work order.

App-kit return AK-CAT-01: local-divergence for the catalogctl CLI task.
Guidance consulted cli.output at catalog revision
1e5ce694e0cbbb1826dcc8f3513b045ec2ec97a0433b2e57f637efd107f47811
and capability revision 6ffa60edf292958c683cc43a85e08ebfaebd61a288462d1b4bf255a0952b22dc.
The authored quality queue reports no cli.output work item, not a quality pass.
The stdlib-only constraint prevents importing dependency-bearing app-kit packages;
plain JSON and its exit conventions are adopted with a documented divergence.

No app-kit source change is proposed. Revisit this divergence if a dependency-free
output contract is published. Agentic-kit owns the local implementation and tests;
no global config or original consumer dependency pins were changed.
