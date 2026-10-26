# Kit independence and interoperability evidence

Date: 2026-10-21. Baseline main: 23a243c23682952bfc6ebcabf90e6b5774ec8110.
Tested source: 3f554730207dd975d2e4b5ec36989cae31d2bbda.
Decision: [0024](../../../intent/decisions/0024-independent-kits-share-identity-not-authority.md).
Acceptance: [kit-interoperability](../../../designs/kit-interoperability.md).
Work order: [KI-1 through KI-6](../../../plans/kit-interoperability.md).

| Instrument | Result and scope |
|---|---|
| Full repository gate | `task validate` passed after the last source changes, exit 0. Includes formatting, vet, CUE fixtures, constitution/shell checks and root/nested race suites. Retained log: `/Volumes/m2ext/tmp/agentic-kit-interop-final-validate.log`. |
| Plan proof | Both interoperability and catalog uplift plans passed `fathom verify --strict --require-all`: 21 total rows, 16 claimed, 16 verified, 0 failing. All six KI code rows and ten existing CK code rows were proven; deferred CK-11 through CK-15 remain deferred. Runtime integration is recorded separately below. |
| Scaffold safety | Existing file, directory, symlink and dangling-symlink collisions refuse before publication. Exclusive creation and injected metadata/open/write/close/remove failures test ownership-aware cleanup and exact retained-path diagnostics. The overwrite defect was reproduced before repair. Cleanup is best effort, not a cross-process filesystem transaction. |
| Catalog identity | Exact stable-key lookup returns detached nodes. Tests cover moves, swaps, replacements, semantic changes, deterministic collections and identity search. Malformed redigested bundles refuse across lookup, compare, decode and decision preparation. A year-10000 timestamp reproduced a silent JSON-hash failure before repair; unencodable bundles now refuse. Baseline controls demonstrated the previous false add/remove diff and missed identity search. Not every new capability test was written before implementation. |
| CLI contract | Actual unused-flag and invalid-value cases failed before repair. Explicit false/zero flags, invalid formats, search bounds, missing explanation IDs and negative packet limits now refuse before I/O with exit 2 and empty stdout. Valid invocation and writer-error contracts remain covered. |
| Domain composition | Actual Review and Env loaders passed the final race-tested replay at platform-repo 821c509035a8bbd98907c83b54372bec70116425. Review composes two validated layers, retaining definition/source-span/hash/origin sidecars and floors. Env retains typed refinement and contradiction authority. Insertion/permutation preserve unrelated stable identities. Log: `/Volumes/m2ext/tmp/agentic-kit-interop-final-pilots.log`. |
| Pilot runner | Hermetic Git fixtures prove immutable-pin replay despite advanced/dirty HEAD, refusal of missing or invalid revisions, inherited Git-context isolation and scratch-path refusal before writes. Final actual-loader snapshot: `/Volumes/m2ext/tmp/agentic-kit-catalog-pilots-6d2mUeor`. Custom runner pins test runner mechanics, not arbitrary consumer compatibility. |
| Independent adoption | `examples/catalog-consumer/run.sh` builds a public-API-only program in an external temporary module with `GOWORK=off`, `GOPROXY=off`, `GOSUMDB=off`. CUE-exported catalog/profile/policy/identity and raw definition/evidence fixtures exercise composition, exact stable lookup, definition/evidence binding and reuse identity. Changed definition refuses without a provider. Root gate tests include this executable path. |
| Paths with spaces | A separate clean checkout of the committed source at `kit with spaces`, using `scratch with spaces`, completed the actual external-module program, exit 0, stable key `review.evidence`. Receipt: `/Volumes/m2ext/tmp/agentic-kit independent proof-jp3m5ee2/receipt.json`. This proves that invocation, not a compiled VCS stamp or measured human setup time. |
| Independent review | Source reviews covered library/CLI behavior, scaffold failure ownership and runner guards. Follow-up found no remaining blocker. The host rejected explicit deep-review routing, so independent standard-tier reviews completed. The optional Fathom composed review had only 3/8 dimensions configured; it is advisory, not a full certification. |
| Downstream aggregate | `task downstream-check` passed at the clean committed source, exit 0, 1,039 s. Log and exit receipt: `/Volumes/m2ext/tmp/agentic-kit-downstream-20261021-q2x9LG/downstream-check-3f55473-07b249c.{log,receipt}`. The following four targets ran in prepared disposable snapshots; original consumer edits were preserved. |
| Fathom | 07b249ca5d7811f9778f3ec111f59e6d029ee0bc passed exact kit mirror/vendor verification, CLI/all-package builds, owned race suite (2,234 CLI tests across four shards), and unchanged performance gate (`ok 32.293s`). Caller GOMAXPROCS was unset; the helper caps only the race task at 2. Its separate app-kit reverse check was explicitly unconfigured/skipped, not a fifth consumer pass. |
| Consumer-a | a007075fab6ac156be44de46df941cce4a0915ed passed configured build and tests. |
| Consumer-b | 8b8620936afd2ebcff6810baec1ef37528699c1a passed configured build and tests using preserved exact ignored prerequisite fixtures. No private fixture data is included in this record. |
| consumer-c | 04be1238bae18e9542b135060233412b4abd72ca re-pinned its scratch copy to `v0.9.1-0.20261022002352-3f554730207d`, re-vendored, built and passed vendor/no-workspace lint guards. This is the configured guard scope, not its entire application test suite. |
| Recovery and cleanup | [Verified backup and disposition](cleanup.md). Fresh origin fetch retained the recorded heads and GitHub reported no open PRs. |

The final publication delta is evidence/tracking only. This does not require
another consumer re-pin or repeat of unchanged runtime checks. Normal commit and pre-push
hooks remain enabled. Publication requires a clean primary tree, fast-forward main,
the full gate, source-backed plan proof and an exact remote-head comparison.

The tests demonstrate bounded implementation and prepared integration. They do not
establish production migration, saved operator effort, model quality, calibration,
universal catalog semantics or compatibility with every future consumer revision.
No provider call, new dependency, permanent public module, tier promotion or domain
authority transfer was introduced. CK-11 through CK-15 retain their explicit
production-migration, future-protocol and measured-evaluation dispositions.

App-kit return AK-CAT-01 remains a local divergence for the stdlib-only catalogctl
CLI. The contribution-loop guidance was retrieved again for this change. The new
command-specific refusals belong to this CLI; no reusable app-kit source defect or
upstream component candidate was identified. Revisit when a dependency-free output
contract is published. The originating implementation and tests are owned here.

Tooling continuity: O11's initial resource refusal recovered, then automatic
post-commit refresh again refused with host swap-volume free 3.4 GiB below its
configured 5 GiB floor. O11 is reopened for host-headroom follow-up and a daemon
refresh after recovery. The guard is retained; independent exact-tree proof and
runtime tests carry publication evidence. This does not resolve the separate O9
synchronous-ingest timeout. Neither a refused nor a timed-out refresh is a proof pass.
