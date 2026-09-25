# Standards and readiness inventory — 2026-09-16

[ADR-0069](../adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md) now
has a CUE-authored implementation in `internal/qualityplan/`. It records seven
standards, the nine existing target gates, five core starter areas and one deferred
personal-site candidate. It does not duplicate the commercial source inventory.

The initial concrete export contained 42 standard entries (32 partial, 10 not-started),
54 gate entries (39 partial, 15 not-run) and one scoped RAC consumer record.
Zero accepted/passed entries are a current observation, not a permanent test constraint.
An empty consumer list means no verified record here, not that an app never used the kit.
The later [shell baseline](2026-09-16-shell-performance.md) moves its performance
standard and gate to partial, retaining failed attempts and broader unmeasured work.
The source hashes below identify the initial inventory, not that later revision.

## Mechanical checks and review repairs

- CUE rejects incomplete accepted items, unreviewed N/A, passed gates without required
  evidence/pins/review, and adopted consumers without a reviewed pinned receipt.
- Source tests compare gate names with `designreceipts`, standard crosswalks, exact
  core IDs, unique work-item IDs and current catalog capability/profile compatibility.
- Reference checks require repository-contained regular files; directory and external
  symlink canaries fail. Contract/receipt hash-drift canaries fail on changed bytes.
- A complete synthetic accepted item passes both shape and file checks. Review removed
  the original blanket refusal that would have prevented the live inventory progressing.
- Starter areas, candidates and selected conversions are distinct. A conversion must
  declare original/target identity, pinned acceptance contract, workload and named
  budgets before active/accepted state. Candidates cannot bypass that selection by
  moving straight to in-progress. Missing contract/budget cases fail.
- MCP round-trips return exact guide/standard/inventory/schema bytes. Both CUE resources
  advertise and return `text/plain`; existing Markdown resources retain their MIME type.

The [workflow](../design/just-in-time-conversion.md) contains commands and update rules.
Selected source, artifact and consumer revision hashes remain authored identities;
the checker does not authenticate their relationship to receipt fields or artifact trees.
Matching receipt bytes and valid CUE are not semantic certification, runtime adoption,
an automatic readiness score or permission to mark a whole goal complete.

## Verification and identity

Concrete CUE vet/export, the seven quality-plan test groups, quality-plan/MCP race
suites, and root `go test ./... -count=1` pass. The root run covers 72 tested packages
plus seven packages with no tests; it excludes separately nested native modules.
Focused theme/Gallery/CLI/catalog race tests also pass. Selected documentation checks
resolve 241 local links, preserve router/living/ADR budgets and paired agent guidance.
These are working-tree checks, not a released candidate or a full `task check` claim.

| Source after verification | SHA-256 |
|---|---|
| `schema.cue` | `de2e1f59797490ba0227f0c5a0651925a0c86a107c1faa3ecd3847503b7d10ca` |
| `inventory.cue` | `ad2e5193abbb4e752e44f59995df04c50d3f5b67c38da1cf519ce30322da5e1c` |
| `quality_test.go` | `fc9dd62ba854267d38ccc20a8a3bd19797f9a919b027fecf14e7d86f7ad96ed4` |

No public runtime API, new server/database, dependency, consumer deployment or automatic
source conversion was added. Subsequent selected conversions must retain their own
exact execution evidence; these inventory checks confer no inherited gate passes.
