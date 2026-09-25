# Connected JIT contribution rehearsal

G4 follow-up under ADR-0066/0069 and I8/I14/I15/I17/I18. This is a synthetic test of
record connections, not execution of a converted component or a consumer upgrade.
The actual inventory, catalog, production checker and public interfaces are unchanged.

## What the example exercises

`TestJITContributionPathRehearsal` reads
`internal/qualityplan/testdata/jit-contribution-v1.json` and constructs an in-memory
selected conversion from the existing deferred candidate. It does not write CUE or
promote that personal-site candidate. The fictional source, target, app and kit
revision hashes are authored test values, not hashes of an executed implementation.

The example joins the work-item ID, selected original/target, pinned contract,
workload/budget, all seven standard decisions and nine gate decisions to a reviewed
receipt and contribution record. The receipt is the synthetic fixture's actual bytes;
its reviewer and asserted successful results are fictional. File integrity is checked
independently; matching hashes do not authenticate the claims inside those bytes.

The contribution has its own ID, observed/expected result, ownership, disposition,
next action/owner/revisit, linked lesson/test and delivery selection. It returns a
local lesson while leaving second-consumer verification and propagation unverified,
and promotion experimental. A test-specific `returned` expectation does not invalidate
ADR-0066's other legitimate dispositions or define a universal contribution schema.

The independent source-membership check requires a `catalog-capability` original to
belong to the selected work item's capability list, which the existing inventory
checks against the actual catalog/profile. Copying the same nonexistent source ID
into both joined records must still fail. This does not authenticate revision hashes.

## First frozen acquisition and review

Private root: `/Volumes/m2ext/appkit-theme-cancel.F2rbr0oq/jit-check-4lf7xN`.
The first revision passes the positive joined example, 17 missing/drift/claim refusal
subtests, and race tests for `internal/qualityplan` and `cmd/appkit-mcp` with Go1.26.3.
Repository-local Go/test/embed sources, the JSON fixture, relevant documents and
resolved Go/Node executable bytes were retained before execution and remained unchanged.
Dependencies use an existing offline module cache; this is not a hermetic release build.

| First acquisition | SHA-256 |
|---|---|
| Receipt | `0b2935e5d00eef3e4385e7ebc5a2e115b18578e5f25ec69f8c26a694da1af508` |
| Test source | `5b24ab9a1456731bb80bab1bdf005e9d3592a818b186d2bc3672c27acd9dfb6d` |
| Synthetic fixture | `5b583502f294f02ef8f590a18d0a25526a98a01ba12e813779aef5aa73469fa3` |

Independent source review found omitted handoff fields and no independent selected-
source membership check in that first passing revision. The repair adds both and
keeps the first packet intact. It cannot retroactively expand the first run's coverage.

## Repaired acquisition

The repaired revision passes the positive example and all 25 rejection subtests:
five missing CUE requirements; pin/review/decision drift; seven missing handoff fields;
correlated nonexistent-source substitution; and revision/disposition/delivery or
unsupported verification/promotion claims. Both package race suites pass again.
Independent source review found no remaining blocking issue in that bounded example.

Private root: `/Volumes/m2ext/appkit-theme-cancel.F2rbr0oq/jit-check-AfLmFL`.
Its acquisition retains the exact source/fixture/document and executable inputs before
running, verifies unchanged inputs afterward, and keeps command arguments and raw
outputs. This assessment's final results paragraph postdates that frozen acquisition.

| Repaired acquisition | SHA-256 |
|---|---|
| Receipt | `9399d2a317fd812d6f36e49ac9273e6504afdd14c1453ac898e3b955f8b7b15f` |
| Test source | `b318adfbf4ddd8b22fc124d86a9deff4b2fa09d1e16f28e4a86fbb8dd7c77534` |
| Synthetic fixture | `9d90edc22e6f3bcd8e678317d3a65a04f8f23ad5e66df1a4b5f49f280e5e413e` |

Replay with `go test ./internal/qualityplan -run '^TestJITContributionPathRehearsal$' -count=1 -v`;
the broader scoped command is `go test -count=1 -race ./internal/qualityplan ./cmd/appkit-mcp`.
These commands run local checks, not a consumer-host or release gate.

## Limits and real next work

No browser task, renderer conversion, CSS delivery, measured budget, actual reviewer,
fork removal or consumer upgrade occurs in this test. It demonstrates authoring and
mechanical refusal boundaries, not the whole required JIT contribution path. The live
readiness statuses and G4 completion remain unchanged.

The [propagation reconciliation](2026-09-16-propagation-reconciliation.md#why-the-complete-g4-goal-is-not-retired)
still requires a real guide/JIT consumer task, exact contract/source identity, retained
task/build evidence and an originating consumer's contribution disposition. RAC has
an app-owned guide; a second guide consumer and reviewed versioned propagation remain
unproven. Preparing or authorizing that work is not an integration result.
