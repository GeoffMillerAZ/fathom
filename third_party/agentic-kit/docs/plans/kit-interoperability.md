# Kit interoperability and independent adoption

Owner request: 2026-10-21. Decision: 0024.
Acceptance: [kit-interoperability](../designs/kit-interoperability.md).

## Implementation

- [x] Preserve existing scaffold outputs and report incomplete cleanup [KI-S1] {id: KI-1, symbol: run, package: cmd/kit-new}
- [x] Verify and select detached nodes by exact stable key [KI-S2] {id: KI-2, symbol: LookupStableKey, package: kits/catalog-kit}
- [x] Preserve moves and replacements in deterministic semantic diffs [KI-S3] {id: KI-3, symbol: Compare, package: kits/catalog-kit}
- [x] Index node IDs and stable keys without replacing the ranker seam [KI-S4] {id: KI-4, symbol: BuildDocs, package: foundations/knowledge/catalog}
- [x] Reject unused flags and invalid command values before input I/O [KI-S5] {id: KI-5, symbol: run, package: cmd/catalogctl}
- [x] Refuse malformed or unencodable bundles across every checked entry point [KI-S2] [KI-S3] {id: KI-6, symbol: VerifyBundle, package: kits/catalog-kit}

KI-S6 and KI-S7 are separately exercised integration deliverables: actual pinned
Review/Env loaders and a Go program built in an offline external module. Their
subprocess/runtime receipts are evidence; static Go reachability does not prove
external execution. TRACKING records these capabilities after the full gate.
KI-S8 is the publication and recoverable-cleanup acceptance; its inventory/backup
receipt names exact refs, paths and object/file integrity checks.

## Verification

KI-1 reopened on 2026-10-24 after native Linux CI deleted a replacement during rollback.
[INC-23 qualification](../evidence/command-tools/2026-10-24-rollback/README.md) closes
the original regression and replacement/edit/failure cases with live ownership pins,
Mac race/full gates and native Linux CI run 40093730634 at `c7633a4`.
Static reachability alone did not close it. Native Windows remains unqualified.

Current evidence: [implementation and integration](../evidence/kit-interoperability/2026-10-21/verification.md)
and [recoverable cleanup](../evidence/kit-interoperability/2026-10-21/cleanup.md).

```sh
go test -race ./cmd/kit-new ./cmd/catalogctl ./kits/catalog-kit ./foundations/knowledge/catalog ./kits/catalog-kit/pilots ./examples/catalog-consumer
bash kits/catalog-kit/pilots/run.sh
bash examples/catalog-consumer/run.sh
task validate
task downstream-check
fathom verify --service agentic-kit --plan docs/plans/kit-interoperability.md --strict --require-all
```

Missing consumers, resource-gated source coverage, unsupported host behavior and
production adoption are never inferred from a passing local scenario. Code plan
boxes are checked only after strict proof of the claimed form and the full gate.
