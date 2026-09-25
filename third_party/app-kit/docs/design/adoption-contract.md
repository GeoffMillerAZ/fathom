# Read-only adoption contract

[ADR-0024](../adr/0024-read-only-adoption-contract.md) defines the boundary.
`inspect` inventories declarations; `check` verifies an explicit consumer-owned selection.
`pins` retrieves candidate metadata; `draft` authors a reviewable v2 selection.
`search` and `capability` retrieve embedded catalog metadata without a host or consumer root.
None executes the app or proves runtime adoption.

## Commands

```sh
appkit adoption inspect --root /path/to/consumer --json
appkit adoption check --root /path/to/consumer --json
appkit adoption pins --root /path/to/consumer --json
appkit adoption router --contract docs/app-kit.md
```

Inspect/check/pins accept root-relative `--manifest PATH` (default `appkit.adoption.json`).
They never discover or mutate sibling projects. Inspection
reports the current catalog revision, optional selection, module declarations and
hashes/presence of root go.work, CLAUDE.md and AGENTS.md. Absent optional files are
not-applicable. Inspection does not compare selected artifacts or capability fit.
Router prints the canonical short marked block for review; it does not install it.
JSON is one deterministic version-1 report with outcome, exit_code, catalog_revision,
normalized selection/hash when available, module observations, file hashes, checks and
limitations. `checker_build` records the inspecting process's main module/version,
Go/OS/architecture and recorded VCS revision/modified flag; absent VCS data stays unknown.
A dirty flag is not a working-tree content hash or the consumer dependency's revision.
No timestamp, absolute root path or file body is included. Module replacement
paths are literal declarations and may contain local paths; do not publish reports blindly.
Default human output uses cli-kit diagnostic tables; NO_COLOR and pipe handling apply.

| Exit | Check | Inspect / pins |
|---|---|---|
| 0 | every selected static check passed | observations completed; not adoption success |
| 1 | observed mismatch, invalid contract or missing required file | malformed/unsafe input; pins also refuses a missing manifest or unknown ID |
| 2 | usage, inaccessible root, read budget, cancellation or incomplete inspection | same operational distinction |

If both a mismatch and an unavailable read occur, exit 2 preserves the incomplete-check
signal and the report retains both findings. Missing selected files are failures, not skips.

### Discover without MCP or Gallery

[ADR-0082](../adr/0082-discover-adoption-capabilities-without-a-host.md) supplies a
JSON-only first-party discovery path in the same CLI:

```sh
appkit adoption search --query comparison --profile go-web --limit 5
appkit adoption capability --id analytics.comparison --expected-catalog-revision REVIEWED_SEARCH_REVISION
```

Search accepts `--query`, `--layer`, `--profile`, `--kind`, `--maturity`, `--limit`
and `--cursor`, with the same catalog semantics as MCP. Pages default to10 items
(maximum50); retain the query, filters and limit when passing `next_cursor` back.
Valid empty matches are empty arrays, not proof that the task has been implemented.
Exact retrieval prints `revision`, full `entry` and the canonical v2
`capability_revision`. Review dependencies, app-owned seams and proof gaps before
using its ID/pin. The optional guard takes the whole-catalog search revision, not a
per-capability pin; stale/malformed values refuse before metadata is returned.
Omitting it reads the binary's current snapshot. Rebuild to refresh that snapshot.

Both commands always print JSON, so neither has `--json`, `--root` or `--manifest`.
They read no consumer files and use no network, model or licensed corpus. Exit0 means
retrieval succeeded (or help), not adoption passed; all failures use exit2. Output
is capped at128KiB before writing; a broken writer may receive partial JSON.
CLI/MCP/Gallery results agree for identical embedded metadata, not necessarily for
differently versioned binaries. These commands do not fetch docs or commercial source.
The [isolated CLI receipt](../assessments/2026-09-23-adoption-cli-discovery.md) records
package/conformance and synthetic actual-process proof, not real-app acceptance.

## Authored selection

The canonical schema is `internal/adoption/schema.cue`. Consumer data is inert JSON,
not executable CUE or a command list. This example is deliberately incomplete until
the catalog revision and file hashes are filled from the reviewed current state:

```json
{
  "schema_version": 1,
  "consumer": "example-app",
  "profile": "go-web",
  "layers": ["L1", "L2", "L3b"],
  "capabilities": ["shell.layout", "shell.appearance"],
  "catalog_revision": "REPLACE_WITH_64_LOWERCASE_HEX_FROM_INSPECT",
  "contract": "docs/app-kit.md",
  "module": {
    "file": "go.mod",
    "version": "v0.0.0",
    "allow_local_replace": true
  },
  "artifacts": [
    {"path": "web/css/theme.css", "sha256": "REPLACE_WITH_REVIEWED_FILE_SHA256"}
  ],
  "agents": true
}
```

Use the actual exact declared module version; the example's development allowance is
not a delivery recommendation. Each selected capability must fit the chosen catalog
profile and at least one selected applicable layer. CLI entries can have no web layers.
Experimental entries remain experimental; a fitting record does not promote its proof.
Unknown IDs/profiles/layers and stale catalog revisions fail instead of silently adapting.
One contract describes one profile; select a distinct manifest explicitly for another surface.

For an authorized revision reconciliation, retain an exact checker/catalog and review
the selected records against the consumer contract and runtime evidence before changing
the pin. If old catalog bytes are unavailable, say so: a prospective re-selection is
not an old-to-new comparison. Keep all other requirements unchanged and replay the
strict check; do not waive a catalog mismatch or automate away the review.
[Fathom's scoped receipt](../assessments/2026-09-14-navigation-delivery.md#prospective-catalog-re-selection)
records this limitation; optional v2 pins below avoid unrelated-record invalidation.

### Explicit version2 capability pins

[ADR-0065](../adr/0065-pin-selected-adoption-capabilities.md) adds opt-in v2; existing
v1 contracts still require the whole-catalog revision. In a separately reviewed v2
manifest, remove `catalog_revision`, set `schema_version` to2 and include:

```json
"capability_revisions": [
  {"id": "shell.layout", "sha256": "REPLACE_WITH_REVIEWED_CURRENT_METADATA_PIN"}
]
```

Required array, including `[]` for no capabilities; exactly one pin per selected ID,
no duplicates/extras. Pins normalize by ID. Each binds the catalog schema version and
complete ordered metadata, not implementation bytes; the ADR fixes the hash format.
Unrelated entries do not invalidate it; selected metadata changes still fail.
All profile/layer, artifact, module and router checks remain.

`pins` prints a separate version1 command report containing `capability_revisions`
and full `entries` for the explicit manifest's IDs. Exit0 / `observed` is only current
metadata retrieval, not agreement, fitness or adoption success. Unknown IDs fail.
No artifact reads, app commands or manifest writes; review before accepting hashes.
Exact MCP `get_capability(id)` and Gallery `/api/catalog/{id}` also return one
`capability_revision: {id, sha256}` alongside the complete `entry` and unchanged
whole-catalog `revision`. All three paths use the same canonical pin function.
Review the entry before copying that object into a v2 selection; never substitute
the whole-catalog revision or hash the wire JSON yourself. A rebuilt server is
required for this additive field; absence means use `adoption pins`, not infer a pin.
When following MCP search, pass its whole-catalog `revision` as optional
`get_capability.expected_revision`. A malformed or changed revision refuses the
lookup; restart search and review the new selection rather than dropping the guard.
Omitting it retains latest-snapshot lookup. This precondition needs a rebuilt server;
it is separate from the returned per-capability pin and does not prove source freshness.
Gallery exact HTML/JSON routes also accept `?expected_revision=<catalog revision>`;
generated capability links include it. Stale selections return409 before entry/pin
content, malformed or duplicate guards return400, and omitted/empty guards keep
current lookup. Recovery retains valid filters/theme but drops the stale cursor/guard;
invalid filters reset together with an explanation. Theme switching preserves a guarded
detail's guard. These are metadata review
controls, not artifact validation ([evidence](../assessments/2026-09-23-gallery-catalog-revision.md)).
Empty capability selections still need an artifact/module/router requirement. Rebuild
before migration; old checkers refuse v2. The [original invocation](../assessments/2026-09-16-selected-adoption-pins.md)
retains v1 failures; the authorized [RAC/Fathom migration](../assessments/2026-09-16-fathom-display-adoption.md#consumer-v2-metadata-migration)
records selected-metadata review and passing v2 static checks, not runtime acceptance.

`module` is optional. Artifact/vocabulary-only adoption need not acquire a Go runtime or
kit.lock. Arrays may be empty when another explicit requirement carries the adoption;
non-CLI profiles still declare applicable layers. Every listed artifact is required and
must match its authored lowercase SHA-256. Hash app-owned aliases/generated CSS as their
own bytes; the hash is not a claim that app-kit generated, tracks or owns them.
No recursive source scan infers which capabilities an app should select.

Module observation reports app-kit's declared require and matching replacement, with
version-specific replacement precedence. Duplicate app-kit declarations fail. Local
replacements require explicit development allowance; remote replacements do not satisfy
the declared kit requirement. No replacement path is followed. Ambient Go environment,
ancestor workspaces, module caches and effective dependency resolution are not checked.
Pin verification here is declaration agreement, not a sibling-free clean-build proof.

When `agents` is true, both root instruction files must contain exactly one matching
block from `adoption router`. CRLF is normalized; unrelated instructions are preserved.
The block points at the required human contract document, whose existence/nonempty
content is checked but whose product/design completeness requires review.
`agents:false` imposes no instruction-file requirement. Neither setting proves a host run.

In that human contract, include a route to the [contribution loop](contribution-loop.md)
(`get_kit_decision("contribution-loop")` or a reviewed pinned copy), its compact
contribution records, and the app's ownership boundaries. This completes the short
router's handoff without changing its checked bytes or copying kit policy. If the
selected docs cannot supply the guide, record canonical review as pending; do not
claim the static checker validates this content or that an agent has read it.

## First-time draft authoring

[ADR-0078](../adr/0078-draft-explicit-adoption-selections.md) adds a CLI-only path
that needs no existing manifest and emits JSON to stdout, never installs files:

```sh
appkit adoption draft --root /path/to/consumer --consumer my-app \
  --profile portable --layer L0 --layer L1 --contract docs/app-kit.md \
  --capability foundation.tokens --artifact public/theme.css
```

Discover and review the exact capability first with `search` and `capability`, or
MCP/Gallery. Optionally pass its whole-catalog
`revision` as `--expected-catalog-revision` to refuse a changed catalog. The draft
contains current per-capability metadata pins and hashes of the explicitly listed
existing files. It rejects duplicate choices, incompatible profile/layers, unsafe
paths, missing artifacts and read-budget failures; no renderer substitution occurs.
Repeat `--layer`, `--capability` and `--artifact` once per value (not comma lists).
Explicit root, consumer slug, profile and contract path are required.

Select `--module-file go.mod --module-version v0.1.0` only for an actual reviewed
requirement; `--allow-local-replace` explicitly permits development replacement.
These are declarations, not resolved or inferred versions. `--agents` requires
both canonical root instruction blocks when subsequently checked; default is false.
Neither option reads or creates its referenced files. A module-only or agent-only
selection is valid; empty capabilities/artifacts are JSON arrays, not null.
Non-CLI selections still need layers, and a wholly empty requirement is refused.

Review stdout before deliberately saving an adoption manifest. Prepare the human
contract and, if selected, review `adoption router` output into both instruction
files. Then run `adoption pins --json` to review the selected metadata and
`adoption check --json` to check the saved contract. Missing contract/module/router
files can make that check fail even when drafting succeeded. No project commands,
dependency resolution, documentation completeness or host activation are proven.

Exit0 means **draft produced**, not check passed; all drafting failures use exit2.
There is no `--json` or output-path option: stdout is always one indented v2 manifest.
Output is stable for identical explicit inputs, catalog and read bytes. Reads are
sequential, not an atomic source snapshot. A writer failure may leave partial output;
do not save it as a valid draft. Never redirect over an existing adoption manifest
to silence drift: use the existing pins/review/upgrade workflow instead.
Rebuild older CLI binaries for this additive command; no manifest migration required.
The [synthetic workflow receipt](../assessments/2026-09-23-adoption-draft.md) retains
actual-process and regression evidence, not real-consumer or host acceptance.

## Bounds and proof limits

Manifest ≤64 KiB; ≤64 capabilities and ≤64 artifact paths; each artifact ≤8 MiB;
all reads ≤64 MiB. Documentation/module/instruction files ≤1 MiB. Paths must be
canonical root-relative UTF-8 without traversal, control characters or platform-specific
separators. Symlinks and special files are refused; only regular consumer-owned files
are read. Unix opens are nonblocking so a swapped-in FIFO cannot stall before its
descriptor type is rejected. No subprocess, network, policy installation or write is involved.
These are bounded sequential observations, not an atomic snapshot of a changing checkout.

Tests seed missing/edited files, incompatible catalog selections, stale revisions,
development/remote replacements, router drift, unsafe paths and unavailable reads.
`TestCheckRequiredArtifactFailures` additionally proves selected-file control → deletion
failure → byte-identical restoration, exact path/detail/remedy and no checker writes.
An unselected neighboring artifact is not reported or required. This synthetic static
contract proof alone does not establish consumer invocation; the assessment above
records explicit read-only checks against two real roots, not installed automation.
CLI tests check JSON/exit parity, no stdout decorations, NO_COLOR, cancellation and
writer failure. These checks run through `task check`; they are not a fleet CI claim.
Clean builds, actual host invocations, rendered task quality, propagation/upgrade receipts
and release-candidate proof remain separate requirements in [adoption](adoption.md).

Existing `appkit vendor --check`/kit.lock behavior is unchanged; its separate recorded-byte integrity check does not prove adoption, or vice versa.
