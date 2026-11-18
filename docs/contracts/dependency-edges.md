# Dependency edges contract

Status: accepted design, 2026-11-18 (ADR-114). The Go and JSON surfaces ship after
review of the public door. See the [design](../design/dependency-edges.md) for the
preregistered admission bars.

## Read and consume

`pkg/lang/deps` reads one file from bytes or a set of paths from a caller-supplied,
read-only, stable, repository-rooted `fs.FS`. A file record names its
repository-relative path, dialect, measurement and completeness, then its direct
edges and unknowns.
`fathom-lang deps` serializes the same records as JSON. Edges and unknowns are
sorted arrays, including when empty. Identical repository inputs produce identical
bytes across runs and checkout paths.

A file is complete only if the reader accounts for every dependency construct in
its dialect and resolves every one. Any unknown leaves the file incomplete. A
reader without a passing admission is unmeasured. Its files remain incomplete
unless the consumer explicitly trusts the dialect by name and the result records
that trust. A dialect with a failed soundness reading is detect-only. A consumer
must widen or run whenever the files in its closure include incomplete evidence.
B2 is inconclusive when a reported local edge in a complete file has an ambiguous
or missing operand identity. The grading harness counts those cases separately
from wrong bindings and extra edges.

A reader may reuse `pkg/lang` reference and unresolved evidence. It does no
network reads, starts no toolchain and computes no closure or hash. It resolves
using the supplied repository view and keeps absolute paths out of every output,
including lexical diagnostics and untrusted extension returns. Raw dependency
operands are kept as portable path tokens when safe to disclose. Other operands
read `***`. Values covered by the library's redaction rule are withheld.

Call `deps.New(opts...)`, then `Reader.ReadFile(ctx, repo, deps.Input{Path: path, Dialect: dialect}, src)` or
`Reader.ReadFiles(ctx, repo, []deps.Input{{Path: path, Dialect: dialect}})`. A blank
dialect infers a built-in reader from the filename. Explicit dialect IDs take
precedence.

| Dialect | Filename inference | Admission |
|---|---|---|
| `go` | `.go` | Unmeasured. The Go fixture checks the grading harness. |
| `javascript` | `.js`, `.jsx`, `.mjs`, `.cjs` | Measured, low coverage (DEP-M1). |
| `typescript` | `.ts`, `.tsx`, `.mts`, `.cts` | Detect-only (DEP-M1). |
| `python` | `.py` | Measured, low coverage (DEP-M2). |
| `dockerfile` | `Dockerfile`, `Containerfile`, either with a dotted suffix, or a basename ending in `.dockerfile` or `.containerfile` (case-insensitive) | Detect-only (DEP-M3). |

The batch sorts paths and rejects duplicates. An unreadable file contributes an incomplete
record and the batch returns an error. A single byte read can use bytes the caller
has yet to store in the repository view. The reader checks every path component
below the supplied root with non-following metadata or parent directory entries.
Built-in reads, resolver reads and local resolver targets all refuse observable
symlinks and components they cannot safely inspect. These checks depend on an
accurate view that stays stable for the read. The CLI uses `os.OpenRoot` for
filesystem confinement.

`WithResolver(id, resolver)` replaces resolver-driven resolution for an
implemented detector. `Resolve(ctx, req)` receives a `ResolveRequest` containing
the repository view, source file, decoded operand, construct kind and source line.
It returns a `ResolveResult` with targets, a typed unknown, or both. The reader
sets the source, line and extension provenance and checks local targets in the
repository view. The override voids the built-in reading for that reader.
`WithDeclaredEdges(edges...)` adds logical edges with declared provenance and
leaves every existing unknown and incomplete verdict in place. A version from a
resolver or declaration comes from a manifest pin or lockfile.
`TrustUnmeasured(id)` records explicit opt-in in `trusted` for an implemented,
unmeasured reader. Detect-only and unimplemented readers stay incomplete.

## Extension and conformance

`Reader.Dialects()` lists the implemented dialect IDs, their construct kinds
and the Reader's measured state. IDs and kinds are sorted. A resolver override
sets the overridden dialect's `Measured` to false. Editing the returned metadata
has no effect on reads.

Load a document dialect with `lang.New(lang.WithDialects(packFS))`. Give that
reader to `deps.WithPack(parser, id, kinds)` and supply the pack ID in
`Input.Dialect` for either read entry point. Close the language reader after
dependency reads finish. Map the pack's existing `cross_file` relation names to
dependency kinds. For example, map `includes` to `deps.KindInclude`. Dependency
kinds are the six values in the table below. The pack owns which constructs it
captures.

The dependency reader checks `repo-path` and `repo-root` targets in the supplied
repository view and reports file or directory edges with `pack` provenance.
A registered resolver can resolve a classified reference and reports
`extension` provenance. Unresolved captures, unclassified cross-file captures,
unsupported strategies and parser failures produce typed unknowns. Pack readers
are unmeasured. Trusting one opts into its coverage and still requires all
reported dependencies to resolve. See the runnable `ExampleWithPack` in
`pkg/lang/deps`.

`pkg/lang/deps/depstest.Check` runs a Reader twice against one repository view
and once against a copy at a second root. Supply independent direct-dependency
truth keyed by file, including an explicit empty row when the oracle found no
dependencies. The helper reports B1 missing local targets, B2 wrong or
inconclusive bindings, B3 byte-identical replay, and counts for coverage,
unknowns and extra edges. Score an unmeasured candidate with explicit trust.
Fixture outcomes are evidence for those inputs. Admission still requires the
preregistered population and bars, and detect-only built-ins remain detect-only.

## Serialized facts

An edge names the importing path, kind, safe raw operand, target, resolution,
version if pinned, source line and provenance. Line is one-based when known
and zero for unlocated evidence. Paths are logical
repository-relative slash paths with portable printable characters and no dot
or parent segments. The directory target `.` names the repository root. An
external ID is `ecosystem:name` with `repo` reserved for repository paths. A version
comes only from a lockfile or manifest pin. Unknown targets carry a typed reason.
An unknown names its file, code, safe raw operand, line and possible evidence
sources. An empty `decidable_by` grants no trust.

| Field | Closed values |
|---|---|
| Kind | `import`, `include`, `embed`, `build-input`, `exec`, `config-ref` |
| Resolution | `file`, `directory`, `external`, `unknown` |
| Provenance | `builtin`, `pack`, `extension`, `declared` |
| Unknown code | `dynamic-import`, `computed-path`, `unresolved-alias`, `missing-config`, `unbounded-glob`, `unsupported-construct`, `parse-error`, `unmeasured-reader`, `external-unpinned`, `read-limit` |
| Decidable by | `toolchain`, `lockfile`, `config-file`, `observation`, `declaration` |

Extending a closed set requires architect review. The public Go API snapshot
and the CUE schema with valid and invalid fixtures hold the reviewed shape.

## Declared-edge exchange

`fathom-lang deps` prints one JSON document with `schema_version: 1` and a
`files` array of `FileDeps`. `-export` instead prints a bare array of declared
edges, for example:

```json
[
  {
    "from": {"ecosystem": "repo", "name": "app/main.go"},
    "to": {"ecosystem": "repo", "name": "lib"}
  }
]
```

The source and each file or directory target use the `repo` ecosystem and the
logical slash path as their name. An external target `ecosystem:name` supplies
the two fields directly. Unknown targets are omitted. Pairs are sorted by source
then target and deduplicated. The caller maps each repository file or directory
to the ecosystem and name assigned by its unit resolver, and reconciles external
IDs with that same inventory. The closure engine rejects a pair whose source or
target is absent from the inventory. A caller checks the file records for
completeness before using a mapped result to skip work.

The exchange is data only. Every edge in the file records discloses provenance.
A pack, registered Go resolver or caller-declared edge list can supply facts.
The extension plan includes a conformance helper for grading readers.
