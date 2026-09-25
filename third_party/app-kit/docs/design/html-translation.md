# Private HTML-to-templ syntax translation

This is the first implementation slice of [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md),
not a certified renderer or an arbitrary React-to-Go compiler. The
[twelve source cases](../assessments/2026-09-13-conversion-pilot.md) were frozen before
implementation. Source-native baselines and behavior adapters remain separate work.
For the explicitly pinned Elements grammar, use the separate
[Elements translator](elements-translation.md), not a flag that widens this rule.
Its receipts do not enter the legacy saved-attempt store described below.

## Translate one exact original

From the app-kit checkout, choose an archive ID and member using `assets-list` and
`assets-files`, then run:

```sh
go run ./cmd/appkit data assets-translate -id ARCHIVE_ID -path EXACT_MEMBER.html
```

`-root` selects the private archive store; `-target go-templ` is the only implemented
syntax target. This command verifies the original archive/member before translating.
By default it writes no files. It never edits originals, installs dependencies, calls
an LLM, executes vendor projects, fetches resources or compiles the resulting component.

One JSON result contains exact source identity, rule implementation SHA, Go/parser/
required-templ versions, diagnostics, and (only when supported) `snippet.templ`, its
SHA and byte-offset source map. The fixed output is package `converted`, component
`Specimen()`. A caller choosing to save the artifact must keep it in a private,
Git-ignored location. Paid derivatives do not become distributable kit primitives.

| Process exit | Meaning | Standard output |
|---|---|---|
| 0 | Bounded syntax translated; **not target compatibility** | One JSON translation |
| 1 | Checked source is unsupported by this rule | One JSON diagnostic result; no artifact |
| 2 | Usage, original verification, I/O or internal failure prevented translation | Empty unless writing the output itself failed |

`go run` reports its child's nonzero code on stderr but itself commonly exits 1;
automation needing exact 0/1/2 should build and invoke the appkit binary directly.
Read `status` and `verification`, not just a shell success. Every result currently
has `verification: "not-verified"`; there is no compatibility-pass state in this schema.

## Save and discover private attempts

Add `-save` to persist a checked attempt. Only this explicit option changes stdout
to `{ "id": "syntax@SHA256", "translation": { ... } }`. Unsupported inputs also
retain their diagnostic result, still exit 1, and never gain a partial artifact.
Storage failures return exit 2 before emitting JSON. Default/`-save=false` output
is unchanged. A failed stdout write can occur after an explicit save has succeeded;
retry is idempotent and discovery can recover the saved ID.

```sh
go run ./cmd/appkit data assets-translate -id ARCHIVE_ID -path EXACT_MEMBER.html -save
go run ./cmd/appkit data assets-conversions -id ARCHIVE_ID -path EXACT_MEMBER.html
go run ./cmd/appkit data assets-conversion -id syntax@SHA256
```

All accept the same `-root`. Discovery also accepts `-status translated|unsupported`,
`-limit` (default 10, maximum 50) and a revision/query-bound `-cursor`. Exact get returns
the original-linked entry, full syntax envelope and fresh `replay` disposition.
Discovery is metadata only, not a fresh scan of receipt files or original ZIPs.

The ignored private store has a separate `translations/catalog.json` plus immutable
`translations/entries/SHA256.json` files. Attempt IDs hash the complete canonical
receipt, including source identity, rule/toolchain, diagnostics and optional output.
The original catalog, ZIPs, archive IDs and downloads are never overwritten. Old and
unsupported attempts stay discoverable; saving identical bytes is a no-op. New files
use private permissions. Keep this directory in the private asset backup, not Git.

Save verifies actual archive membership and exact current-rule replay. Exact get
checks canonical JSON, immutable identity, metadata agreement and original membership.
`matches-current` means that replay passed now; `stale-transformer` retains a readable
same-schema historical attempt with a different rule/toolchain identity. Both remain
`not-verified`. A changed/rehashed output under the current rule fails exact access.
Other schema versions or converter families need an explicit reader/migration, not
silent reinterpretation. Current emission/replay still require the exact pinned
dependencies; the separate stored-envelope schema admits historical version identities.

Reads/writes reject symlink or nonregular store entries, malformed/oversized data,
duplicate/aliased JSON keys and conflicting immutable destinations. Catalog bounds
are 4096 entries/16 MiB; receipt JSON is bounded to 32 MiB before decoding, with
pre-allocation array/depth limits. The catalog is published last under an exclusive
lock. A crash may leave an abandoned lock or unlisted immutable file; inspect before
removing a lock. Files are synced, but power-loss/directory durability is not claimed.
There is no automatic deletion, dependency execution, compatibility promotion or
browser/MCP write action. Gallery [source comparison](private-references.md#compare-a-saved-attempt)
now browses these same attempts with independent original/artifact line panes.

MCP `list_design_conversions` uses the same metadata/revision as CLI discovery; its
optional filters are `archive_id`, `path`, `status`, `limit` and `cursor`.
`get_design_conversion` takes an exact attempt `id`, checks original/receipt integrity
and reports current replay or stale identity. Artifact text uses `start_line`/`limit`
(default 1/100, maximum 400 lines and 32 KiB); diagnostics use `diagnostic_start`/
`diagnostic_limit` (zero-based, default 0/16, maximum 64). Continuations are explicit;
oversized windows fail without truncating source. Combined JSON is capped at 64 KiB.
Source-map count is returned; use CLI exact get for the complete envelope/mappings.
Unsupported attempts have no artifact and reject nondefault artifact paging. Both
tools are read-only and never create a store. The exact original is still available
through `get_design_asset_file` using `entry.source.archive_id` and `entry.source.path`.

## Deliberately narrow grammar

The implementation accepts bounded HTML fragments with allowlisted native HTML/basic
SVG and literal attributes. It emits text and attribute values as quoted Go constants,
preserving decoded whitespace and class ordering/duplicates. Source-byte comments
keep templ formatting from inserting spaces between adjacent inline nodes. Ordinary
HTML comments remain inert comments; original source bytes remain separately available.

Scripts, style blocks/attributes, custom elements, known behavior-expression attributes,
unsupported URL forms, duplicate attributes and unsupported syntax yield diagnostics,
never partial output with the difficult parts removed. Non-HTML members explicitly
require a reviewed adapter or their source-native profile. Unsupported does not mean
the vendor source is defective; it means this rule has not earned that grammar/runtime.

Bounds: 256 KiB UTF-8 source, 20,000 tokens, 128 nesting/attributes, 4 MiB generated
source, 64 reported diagnostics plus an omitted count. Token/source offsets are bytes.
Do not infer browser-safe application behavior from this conservative syntax filter.
Generated components still need a reviewed host, CSS, assets, routing and data contract.

## Reproducibility and evidence

`internal/designconvert/schema.cue` owns this versioned syntax envelope, separate from
the existing catalog schemas. `VerifyReplay` regenerates under the current rule and
toolchain and rejects changed source, output, source map, diagnostics or rule identity.
Reformatting or adapting output changes its identity; it is not the same artifact.
Tests include changed/rehashed dark classes and fabricated verification claims.
Replay proves transformation consistency, not archive membership or authenticity;
callers must verify the exact original, as the CLI does before translation.

Ordinary tests use synthetic sources. The opt-in private pilot reads the exact twelve
roots, verifies hashes, and compiles/renders supported constant-only outputs in private
temporary fixtures before and after templ formatting. It compares namespace-aware DOM,
text, attributes and comments; no vendor JS, project installation or network is involved.
Only identities, counts and diagnostic codes are logged, not source or compiler output:

```sh
APPKIT_PROBE_CONVERSION_KIT_ROOT=/absolute/path/to/app-kit go test ./internal/designconvert -run '^TestFrozenConversionPilotLive$' -count=1 -v
```

Syntax translation, compiler/render checks, visual fidelity, interaction, runtime fit
and consumer adoption are distinct evidence. No translation is automatically inserted
into verified-target discovery. Original CLI/MCP/Gallery records remain unchanged;
private syntax-attempt persistence and CLI discovery now link originals to their results.
Gallery source comparison is implemented; rendered comparison and target-compatibility receipts remain work.
The [saved pilot assessment](../assessments/2026-09-13-saved-conversions.md) records exact private attempt IDs and current evidence.

The [full syntax diagnostic](../assessments/2026-09-13-html-syntax-audit.md) accounts for
657 inputs: 439 translate, 217 are grammar-unsupported, one exceeds the size limit.
The separate [compiler/DOM audit](../assessments/2026-09-13-html-compiler-dom-audit.md)
proves all 439 accepted cases before/after formatting, with private task-owned caches.
Both assessments include opt-in commands and metadata-report controls. Neither adds
saved derivatives, target certification or visual/interaction proof to discovery.
