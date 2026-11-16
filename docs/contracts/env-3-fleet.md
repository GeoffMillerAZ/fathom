# ENV-3: generic deterministic fleet contract

Historical pre-W16 status (provenance only): **DRAFT, separate critic required before approval or publication handoff**.
Contract only. No implementation, materialization, benchmark, native qualification,
admission, plan completion or dispatch is claimed. This is the distinct ENV-3
unit under T6, not a repair or approval of the held ENV-1/ENV-2 contracts.

## W18-1/W18-2 unpublished keyed-inventory peer source refresh

Normative schema source: `docs/contracts/env-1-core.md` at
`24904c52a66ec9703522cca60b4e14fc939f4eea`,
SHA-256 `f63a59fe2bfee66b0c102698749eac2a043a99bec5e0cbdfcda17590dfe2f07e`.
T12-1 adds typed inventory expectations in the pinned core; no fleet API,
GeneratorVersion, protocol, bytes, fixtures or native port changes. W16-2 and
T3-1 govern the single inventory and Card.ID join; W3-4 governs the
named generator version. W2-3 requires independent LAND for this exact amendment.
W18-1/W18-2 govern one keyed CUE concept struct in one inventory file, with
T12-1/FIX1 expectations per value and no redundant authored key/id. The Go
projection is map[string]ConceptRecord over EVALUATED CUE field names, not raw
source label spellings; empty inventory JSON uses concepts:{}.
Normal correction1 follows existing CUE field semantics: NFD e\u0301 evaluates
to NFC \u00e9 as a field name; string reference values are not normalized and
must equal the actual decoded key exactly. Compatible declarations may collapse;
conflicting30/31 content at that evaluated field refuses in either order. No ENV
normalizer, alternate identity rule or reference rewriting is added. The stored
invalid.cue negative stays fully commented; tests extract/uncomment its exact
witness into owned scratch for explicit vet/source-validator refusal. Core's
CUEFieldSemantics and CommentedNegative tests pin these exact cases.
CUE/schema plus the valid-invalid pair and cue vet pin conflicting content;
compatible repeated declarations may unify. One exact-file lane path grant,
enforced by pathcheck, owns the inventory. It needs nothing from the record id
checker. Other authored ENV records retain their existing record-id laws.
W18-3 disposition: the existing data shape/schema/checks suffice; use them,
without new machinery. No classifier, CUE-literal discovery, opaque-key grants,
numeric policy or RecordOwnership prerequisite applies to concept keys. The
shared concept-record extension is deferred to REC-6 under its existing owner.
Historical source pins below remain provenance, not normative alternatives.
The preceding W16 core was `2ada35b791d8115fc05e563ba3af571e624a9895`,
SHA-256 `2ac44ac36988aacc9783e76e676b5d2abdd5006afb4ded36241f775fb7047981`
(provenance only).
Earlier core approval covers only its old exact blob. These W18/T12 bytes require
fresh independent review and a new handoff; no F1 closure is claimed.
This amendment supplies no implementation grant, inventory data or path grant,
runtime/native evidence, gate result or plan-row proof. Other authored ENV
records retain their separate REC discovery/grants prerequisite.
Integration remains HOLD pending the controller's exact-source final review.

T6-2 materialization under gitignored workspace and the neutral `.invalid`
Git identity remain unchanged, as do the generator signature and all wire bytes.

## Authority, intent and source identity

[ENV-3](../plans/bench-environments.md), Contracts and independent APIs, grants
`internal/benchfleet/**` for a generic generator depending on ENV-1's schema alone.
[BE-D5 and Sequence](../design/bench-environments.md) require fixture conservation,
repeatable bytes/DAGs, seed-derived truth before ingest and a checker-independent
API. ENV-G1 to ENV-G7 own presets. ENV-C5 alone joins generation, independent native
receipts and coverage. Neither a withheld checker nor a CLI implementation is an
input to this API. No dependency cycle is permitted.

The [intent router](../intent/README.md), P2/P3/P15/P17 in
[principles](../intent/principles.md), and I43/I54/I56/I62 in
[invariants](../intent/invariants.md) apply: deterministic evidence, visible typed
refusals, writes confined to owned paths, computed status, no automatic downloads
and no model-generated proof. Their stated enforcement limits remain unchanged.
The benchmark evidence trigger was already raised in the benchmark design's
Intent before design paragraph. This draft establishes no new ADR-109 revisit
trigger, changes no compatibility surface, and builds no documented future.
ADR-108/109/110 remain proposed records in the [decision index](../intent/decisions/README.md).
The `bench-preparation` exploration map routes to ENV's independent APIs and the
[docs proof rules](../AGENTS.md). It is navigation, not evidence of delivery.

Approval provenance supplied to this lane:

* W2-3: approval by rule requires a separate LAND critic and governing citations.
* W3-2/W3-4: implementer-owned semantic RED, exact named assertions, unchanged
  named enums/JSON keys, typed refusals and nonnil result slices.
* T6-2 approves the exact ENV-1/U6 recommendation submitted in ESCALATE-2, with
  materialization **only below gitignored `workspace/`, never committed**, and a
  **neutral generated Git identity on `.invalid`**. These conditions apply to
  every positive, negative and proof fixture, including fixtures with `.git` data.
* T6-1 reserves fingerprint hashing and canonical fingerprint bytes to BEN-9.
  This generator's SHA-256 integrity checks and SHA-1 Git objects are distinct
  approved purposes, not another fingerprint or coverage oracle.

The operative U6 protocol and fixtures are reproduced below. Ruling identifiers
identify provenance, not external documents an implementer must find to use it.
Ordinary file/helper/test spelling below implements these settled choices. No
unsettled policy choice is proposed, no future field or new module dependency is
introduced, and no held finding is repaired.

| Identity | Exact value |
|---|---|
| Draft base and origin/main at authoring | `54bca03154ea35c39bf08acba4c3572e843da56a` |
| Base tree | `42b0cd95d348450abc8c9258b926127a7902c92a` |
| Historical full-contract source (provenance) | `44d4f45184996c505e0bcc96d5ab5a8c65da56e4`, `docs/contracts/env-1.md`, fleet declarations 1046-1154, U6 1156-1256, fixture/tests 2130-2197 and 2450-2453 |
| Immutable source file SHA-256 | `386977629e4c74e34ba63e9d361efc157e30fc98bd4f91871556d3afef2974a1` |
| Historical extracted core publication (provenance) | `cccb365ae4479c6ceec3c7c57ced83c663a79789`, `docs/contracts/env-1-core.md`, Fleet data declarations and pure manifest API |

The historical extracted core file SHA-256 is `1e965b2fe8d73c5a9ded7d7a5a0bec7c05530964be215e676196388681863b83`.

Neither source commit is stacked on this branch. Their identities bind text
conservation, not a need to import held commits. The normative amended data contract pinned above is reused exactly.
The historical fleet blocks differ only by the W3-4 named-version correction.
Other authored ENV records retain REC ownership upstream. W18 concept struct
validation/path ownership has no REC prerequisite and adds no fleet dependency.
The generator needs its frozen fleet schema API, not excluded checker/CLI behavior
or the full held contract. Core publication review and source integration remain
separate obligations before implementation dispatch.

## Implementation ownership and dependency boundary

This documentation lane owns only this new file and local workspace artifacts.
The future ENV-3 lane owns these new files, all inside its granted package:

| File | Responsibility |
|---|---|
| `internal/benchfleet/generate.go` | `GenerateBenchmarkFleet`, staged validation, checked accounting, canonical result assembly and seed expansion |
| `internal/benchfleet/git.go` | Raw blob/tree/commit encoding and SHA-1 identities, repository refs and HEAD checkout |
| `internal/benchfleet/materialize.go` | Workspace/ignore/containment guard, exclusive staging and publication, cancellation and bounded cleanup |
| `internal/benchfleet/generate_test.go` | Every named test below and package-private deterministic filesystem/cancellation fault injection |

Only the standard library and `schema/benchenv` are required by the implementation.
No `internal/benchenv`, command package, `internal/mcp/costtest`, store, client,
connector, ingest, compiler, evaluator, network, model, scheduler or external CLI
is called or imported. In particular no Git executable is required to generate
Git objects. Reuse [BuildFleet](../../internal/mcp/costtest/fleet.go)'s copy/explicit
bulk-generation pattern only. Do not reuse its store/client model or ingest step.
No test imports that harness. Tests may independently inspect raw Git objects.

Schema types/validators/errors stay in `schema/benchenv/validate.go`, owned by the
approved core lane. They are not copied into the implementing package, aliased,
extended or reimplemented. The following reproduction makes the contract usable
without an excluded ENV contract. It does not grant schema edits.

## Unchanged schema inputs, results and errors

Each hash below is SHA-256 of the exact reproduced UTF-8 declaration block,
including one final LF and excluding Markdown fences. The fleet block includes
its original comment and pure validator signature. The auxiliary blocks and fleet
CUE block are exact against the normative amended core pinned above. The
historical source identities conserve the other bytes; they do not override the
named-version amendment. The generator signature below remains unchanged.

| Core block | Bytes | SHA-256 |
|---|---|---|
| ExclusionKind | 220 | `62f5c83009fef55c477f1d9faa4c3352100280b6528d539857a2b68b4f923a92` |
| Ref | 110 | `9839b1dbec1fd439967fb28bfcf1c3f054ecc9c840b25120adde209c8ddc56ce` |
| ExcludedCount | 129 | `af14417cb2d632e2426d1f9094cb3fb89ce7618d9088b29730e2913b1e277f13` |
| fleet declarations | 3194 | `7ca5587b9f1e9e4e239fcd14a28d8fd9e7215c9a6ef02b3de84009c1ac631e48` |
| fleet CUE | 973 | `4cb7f214de8e1a7c8224548e396e2db4db8dee4727a4173a681cbe18816f88e3` |

```go
type ExclusionKind string
const (
    ExclusionVendored ExclusionKind = "vendored"
    ExclusionGenerated ExclusionKind = "generated"
    ExclusionTest ExclusionKind = "test"
    ExclusionBuild ExclusionKind = "build"
)
```

```go
type Ref struct {
    ID string `json:"id"`
    Path string `json:"path"`
    SHA256 string `json:"sha256"`
}
```

```go
type ExcludedCount struct {
    Kind ExclusionKind `json:"kind"`
    Lines int64 `json:"lines"`
    Files int64 `json:"files"`
}
```

```go
// Proposed schema/benchenv/validate.go, except the named leaf signature.
type FileMode string
const (
    FileRegular FileMode = "100644"
    FileExecutable FileMode = "100755"
)
type FileClass string
const (
    FileProduct FileClass = "product"
    FileVendored FileClass = "vendored"
    FileGenerated FileClass = "generated"
    FileTest FileClass = "test"
    FileBuild FileClass = "build"
)
type FleetFile struct {
    Path string `json:"path"`
    ContentID string `json:"content_id"`
    SourceSHA256 string `json:"source_sha256"`
    SHA256 string `json:"sha256"`
    Mode FileMode `json:"mode"`
    Class FileClass `json:"class"`
    Seeded bool `json:"seeded"`
}
type FleetCommit struct {
    ID string `json:"id"`
    Parents []string `json:"parents"`
    Files []FleetFile `json:"files"`
}
type FleetRepository struct {
    ID string `json:"id"`
    Head string `json:"head"`
    Commits []FleetCommit `json:"commits"`
}
type FleetSize struct {
    Lines int64 `json:"lines"`
    Files int64 `json:"files"`
    Repositories int64 `json:"repositories"`
    History int64 `json:"history"`
}
type GeneratorVersion string
const (
    GeneratorVersionV1 GeneratorVersion = "env-fleet/v1"
)
type FleetManifest struct {
    Seed uint64 `json:"seed"`
    GeneratorVersion GeneratorVersion `json:"generator_version"`
    Template string `json:"template"`
    Size FleetSize `json:"size"`
    Repositories []FleetRepository `json:"repositories"`
    Truth []FleetFile `json:"truth"`
}
type FleetLimits struct {
    MaxRepositories int64 `json:"max_repositories"`
    MaxCommits int64 `json:"max_commits"`
    MaxFiles int64 `json:"max_files"`
    MaxBytes int64 `json:"max_bytes"`
}
type FleetCommitResult struct {
    ID string `json:"id"`
    Commit string `json:"commit"`
    Parents []string `json:"parents"`
}
type FleetRepositoryResult struct {
    ID string `json:"id"`
    Head string `json:"head"`
    Commits []FleetCommitResult `json:"commits"`
}
type FleetResult struct {
    Repositories []FleetRepositoryResult `json:"repositories"`
    Truth []Ref `json:"truth"`
    Size FleetSize `json:"size"`
    Excluded []ExcludedCount `json:"excluded"`
}
type FleetReason string
const (
    FleetShape FleetReason = "shape"
    FleetPath FleetReason = "path"
    FleetDuplicate FleetReason = "duplicate"
    FleetParent FleetReason = "parent"
    FleetCycle FleetReason = "cycle"
    FleetDigest FleetReason = "digest"
    FleetSizeMismatch FleetReason = "size_mismatch"
    FleetTarget FleetReason = "target"
    FleetIO FleetReason = "io"
    FleetCancelled FleetReason = "cancelled"
)
type FleetError struct {
    Path string `json:"path"`
    Reason FleetReason `json:"reason"`
}
func (e *FleetError) Error() string
type FleetLimit string
const (
    LimitRepositories FleetLimit = "repositories"
    LimitCommits FleetLimit = "commits"
    LimitFiles FleetLimit = "files"
    LimitBytes FleetLimit = "bytes"
)
type FleetLimitError struct {
    Path string `json:"path"`
    Kind FleetLimit `json:"kind"`
    Limit int64 `json:"limit"`
    Observed int64 `json:"observed"`
}
func (e *FleetLimitError) Error() string
func ValidateFleetManifest(source []byte) (FleetManifest, error)
```

The fleet CUE projection uses unchanged core helpers `#Text: string & =~"\\S"`,
`#Digest: string & =~"^[0-9a-f]{64}$"` and
`#Int64: int & >=0 & <=9223372036854775807`. No authored result has a pass flag.

```cue
#FleetID: string & =~"^[a-z][a-z0-9-]*$"
#FleetFile: close({
    path!: #Text, content_id!: #FleetID
    source_sha256!: #Digest, sha256!: #Digest
    mode!: "100644" | "100755"
    class!: "product" | "vendored" | "generated" | "test" | "build"
    seeded!: bool
})
#FleetCommit: close({id!: #FleetID, parents!: [...#FleetID], files!: [...#FleetFile]})
#FleetRepository: close({id!: #FleetID, head!: #FleetID, commits!: [#FleetCommit, ...#FleetCommit]})
#FleetSize: close({lines!: #Int64, files!: #Int64, repositories!: #Int64, history!: #Int64})
#GeneratorVersion: "env-fleet/v1"
#FleetManifest: close({
    seed!: int & >=0 & <=18446744073709551615
    generator_version!: #GeneratorVersion, template!: #Text, size!: #FleetSize
    repositories!: [#FleetRepository, ...#FleetRepository]
    truth!: [#FleetFile, ...#FleetFile]
})
#FleetLimits: close({
    max_repositories!: #Int64 & >0, max_commits!: #Int64 & >0
    max_files!: #Int64 & >0, max_bytes!: #Int64 & >0
})
```

The only leaf entry point, preserved from U6, belongs to package `benchfleet`.
`benchenv` below imports the repository's `schema/benchenv` package. `context` is
required because this call writes files. Sources are caller-only raw bytes, never
receipt/diagnostic output. Root is caller-only native input, never a public ref.

```go
func GenerateBenchmarkFleet(ctx context.Context, root string,
    manifest benchenv.FleetManifest, sources map[string][]byte,
    limits benchenv.FleetLimits) (benchenv.FleetResult, error)
```

`ValidateFleetManifest` remains a pure shape/path/identity/DAG call. It cannot
check source-byte equality, measured size, runtime caps, roots or I/O. Missing
raw bytes are never a schema error. Its failure result is exactly:

```json
{"seed":0,"generator_version":"","template":"","size":{"lines":0,"files":0,"repositories":0,"history":0},"repositories":[],"truth":[]}
```

The generator invokes that same validation again, preserving its typed errors
and original input-index pointers, before checking raw sources. Do not implement
a second manifest validator in `benchfleet`. Direct Go inputs with nil required
slices serialize as null and are invalid shape, not silently repaired inputs.
A schema-valid raw-free manifest stays valid without any template lookup.

## Settled expansion, snapshot, accounting and Git protocol

These are U6's exact choices approved by T6-2, with its storage/identity conditions.

1. **Full snapshots.** Every commit declares its complete file set. There are no
   inherited-file operations. Empty root snapshots are valid. Empty/non-UTF8
   source bytes are valid. `sources[ContentID]` supplies every file/truth byte.
   The family `Template` is an explicit nonblank label, never a discovery key.
   Version is exactly `env-fleet/v1`. No default seed, PRNG, hidden preset, source
   registry or disk fallback exists. An absent lookup, including an unknown
   ContentID, is the specified digest refusal. A present nil or zero-length byte slice is
   available empty content; use the map lookup boolean to distinguish absence. Unreferenced raw-map entries are not read, emitted,
   counted or discovered as templates. They cannot influence a result.
2. **Exact expansion.** `SourceSHA256` is SHA-256 of the raw source bytes.
   `Seeded=false` copies every byte, including literal `@ENV_SEED@` markers.
   `Seeded=true` requires at least one exact ASCII `@ENV_SEED@` and replaces ALL
   occurrences with unsigned decimal `Seed`, no padding, newline or other
   substitution. `SHA256` is SHA-256 of the resulting bytes. Truth follows this
   same per-file law, with its own supplied source/output digests. Both supplied
   digests must match. No encoding conversion or line-ending normalization occurs.
3. **Identity and ordering.** Repository, commit and content IDs match
   `[a-z][a-z0-9-]*`. Repository/commit IDs are scoped as the core declares:
   repository IDs fleet-wide, commit IDs within a repository; reuse of a ContentID
   across snapshots is expected. Repository results sort by ID. Commits use
   topological order, selecting the smallest UTF-8 ID among currently ready
   commits. Snapshot/truth files sort by slash path. Parent order is authored
   and retained. No duplicate parent, missing parent, cycle, missing Head or
   commit unreachable from Head is allowed. Input permutations affect error
   indices but never successful file bytes, Git identities or result JSON.
4. **Measured size.** Count only HEAD product files for `Size.Lines/Files`:
   lines = LF bytes plus one iff nonempty and not LF-terminated, including blank
   lines and binary bytes. Repositories = declared repository count. History =
   sum of reachable unique declared commits in each separate repository. Same
   hashes across repositories do not collapse their histories. All four measured
   values must equal the declared size. Zero is known zero, not an S claim.
   Each of vendored/generated/test/build is reported once in that exact order,
   including zero, using HEAD files only and the same line law. External truth
   enters neither scale nor excluded counts. This byte accounting establishes
   no language-aware concept count, adequacy or environment independence.
5. **Caps.** Every `FleetLimits` value is explicit, positive and int64-bounded.
   Count repositories, ALL commit snapshots, file entries in ALL snapshots plus
   truth, and expanded bytes across ALL those entries, including repeats.
   Limits exclude Git encoding/compression overhead. Check in order repositories,
   commits, files, bytes. Equality succeeds, observed greater than cap refuses
   with `*FleetLimitError`, exact Kind/Limit/Observed and
   `/limits/max_repositories`, `/limits/max_commits`, `/limits/max_files` or
   `/limits/max_bytes`. Zero/negative caps are `FleetShape` at that limit pointer.
   Checked arithmetic never wraps, truncates or saturates. Any accounting overflow
   is `FleetShape` at `/size`, before materialization. There is no scaling, retry,
   implicit cap, success prefix or partial-success field.
6. **Git objects.** Object ID = lowercase SHA-1 of
   `<type> SP <decimal byte length> NUL <payload>`. Blob payloads are exact expanded
   file bytes. Tree entries are `<mode> SP <basename> NUL <20 raw OID bytes>`.
   Directory mode is `40000`; regular/executable modes are `100644`/`100755`.
   Sort using Git basename byte order, treating a directory as if suffixed `/`.
   Recursively construct directory trees from the complete snapshot. Empty-tree
   OID is `4b825dc642cb6eb9a060e54bf8d69288fbee4904`. No symlink, submodule,
   filter, hook, attributes execution or host-config object format is involved.
7. **Commit payload.** LF only, exactly the following concatenation, with no
   extra headers, signatures or trailing blank line. Each parent is its full
   commit hash in authored order. `k` is the zero-based canonical topological
   ordinal WITHIN this repository, `t=946684800+k`.

   ```text
   tree <tree-oid>\n
   parent <parent-oid>\n                 (zero or more, authored order)
   author Environment Fixture <fixture@example.invalid> <t> +0000\n
   committer Environment Fixture <fixture@example.invalid> <t> +0000\n
   \n
   env-fleet/v1 <repo-id> <commit-id>\n
   ```

   The displayed `\n` denotes one LF byte, and annotations are not payload.
   Generated author and committer are BOTH the neutral fixture identity above.
   Source-repository commits keep the owner's identity. No global Git config,
   clock, timezone, signing identity or process-global environment mutation enters
   object identity. Write a non-bare SHA-1 repository with symbolic HEAD
   `ref: refs/heads/main\n` and `refs/heads/main` containing the declared Head hash
   plus LF. Check out only that Head's full regular/executable snapshot. No author
   identity or timestamp is taken from the source repository.
8. **Output/result.** Repositories live at `<root>/repos/<repository-id>` and truth
   at `<root>/truth/<file-path>`. Truth is written before publication, outside
   every checkout and before any possible caller ingest. Caller ingest is allowed
   only after successful return and remains outside this package. Success returns
   canonically ordered IDs/full hashes. `FleetRepositoryResult.Head` is a full Git
   hash. `FleetCommitResult.Parents` contains full parent hashes in authored order,
   not logical IDs. Truth refs are `{id:ContentID,path:"truth/"+Path,sha256:SHA256}`
   in path order. No absolute path, source byte map or root leaks to output.
   Every returned collection, including root Parents and empty Excluded rows'
   surrounding list, is nonnil. Loose-object compression, pack layout, filesystem
   inode/mtime and staging basename need not match. Checkout/truth bytes, file
   modes, Git object IDs/DAG and canonical result JSON MUST match.

## Workspace, ignore, containment and publication refusal

T6-2 adds a storage constraint to U6, not another generator field. A caller must
prepare an owned, existing real parent directory strictly beneath its checkout's
real `workspace/`. The generator derives the containing checkout by walking
ancestors of that parent to its nearest `.git` directory or regular worktree
`.git` marker. It uses native path operations, no CLI or coverage/checker service.
Relative root input is resolved once against the caller's working directory,
without changing it. Public paths below remain slash paths, never native roots.

The bounded ignore guard recognizes this repository's existing convention:
root `.gitignore` contains the literal anchored `/workspace/` directory exclusion.
Blank/comment lines are ignored. Any later negated pattern conservatively makes
this guard refuse rather than implementing another general ignore engine or
assuming a global exclude. An ignored parent cannot be re-included by a nested
ignore file. This sufficient local check needs no Git configuration or external
command. An equivalent but unsupported ignore spelling is a visible target
refusal, never silent output elsewhere. No ignore file or index is rewritten.
This is a conservative executable realization of T6-2 for the existing
[workspace convention](../../AGENTS.md) and root `.gitignore`, not an alternative
materialization protocol. Separate review must check this guard against T6.

Before any owned staging write, require all of the following. Every failure in
this paragraph is `*benchenv.FleetError{Reason: FleetTarget, Path: "/root"}`.

* A containing checkout marker, the effective local exclusion above, and an
  existing directory `workspace/` can be established without following symlinks.
  An unreadable guard cannot establish containment and refuses. The root's parent
  already exists, is a directory, and is strictly inside that workspace, or is
  workspace itself. Do not create missing caller ancestors.
* The requested root is a nonempty clean native path strictly BELOW workspace,
  not workspace itself, a sibling prefix such as `workspace-other`, checkout root,
  a source/fixture directory, or an arbitrary external temporary directory.
  Do not normalize `..` traversal into permission to write. A root with an empty,
  `.` or `..` final component refuses.
* `Lstat` detects any existing root, including regular file, empty/nonempty
  directory and dangling symlink. Reject symlink components in the containment
  chain even when their targets remain inside workspace. Parent files, escaping
  symlinks and unresolved/changed containment refuse. Compare path components,
  never a raw string prefix. Recheck before publication.

Manifest file/truth paths are nonempty canonical relative slash paths. Reject
absolute/drive/colon-bearing paths, empty components, `.`/`..`, repeated/trailing
slashes, backslashes, NUL/control bytes and any `.git` component, as well as duplicate
file paths and file/directory-prefix collisions within a snapshot or within truth.
Validate before joining with native paths. Different snapshots may deliberately
change a path from file to directory because snapshots are independent. Native
filesystem aliasing that would map distinct validated paths onto one output, or
onto Git metadata, is a target refusal at `/root`, never a successful overwrite.
That check belongs to materialization, without widening the pure core schema.

Write to an exclusively created private sibling under the SAME ignored parent,
with only this call owning its staging subtree. Publish the fully written fleet
by rename only after objects, refs, Head checkouts, truth and result are complete.
Publication must reserve/refuse the target exclusively, including a target created
between preflight and publication. An empty target reservation created by this
call after staging completes may be used to support exclusive publication, but a
caller-owned target is never replaced or removed. A collision is FleetTarget at
`/root`. Caller mutation of staging/reservation contents is outside ownership.
Do not copy a partially written fleet to the result path.

Cancellation before/during writes or before publication returns FleetCancelled at
`/root`. Check context at bounded work boundaries, including between files and
before publication. Pre-cancelled valid input performs no filesystem mutation.
Ordinary staging/object/ref/checkout/truth/write/chmod/rename failures return FleetIO
at `/root`. A discovered target/containment collision remains FleetTarget. Preserve
these exact exported error types/reasons, not strings or a mode-error wrapper.
No new error fields or methods are added. An I/O cause is not promised through an
undeclared `Unwrap`. Source bytes/native paths never appear in diagnostic output.

On cancellation/I/O/collision, remove only this call's owned staging and any owned
publication reservation, leave no successful result root, and retain a caller's
preexisting parent, keys, roots and sibling sentinels unchanged. No blanket cleanup,
parent deletion, chmod of caller paths, global-config edit or automatic retry is
authorized. Tests inject bounded faults in owned operations, not permission changes
to real shared directories. Once publication succeeds, ownership of the finished
root transfers to the caller. The API never stages/adds/commits that fleet to the
source repository. Any test Git/control metadata also remains under workspace.
The repository stores generator sources, seed inputs and expected digests only.

## Error precedence and initialized result law

Shape precedes semantic manifest validation. Within each, collect candidates and
select the smallest RFC6901 pointer by UTF-8 bytes, then serialized reason text.
A missing parent reports that parent's pointer, no cascaded errors from unavailable
targets. First ID occurrence defines it, subsequent occurrence is duplicate.
Pointers retain ORIGINAL input array indices, even after successful canonicalization.
Pointer escaping is `~` to `~0`, `/` to `~1`. Malformed/nonconcrete root is `""`.

After that succeeds, check source/expansion/digests and measured-size equality,
using the same pointer/reason order for independent errors in that stage. Then
check positive caps and ordered cap exhaustion, valid root/guard, and finally
cancellation/materialization. For a valid manifest/source/caps/root, cancellation
has its exact typed result. Do not promise which asynchronous error wins if a
context changes at the same instant as a failing I/O operation. Deterministic
fault tests trigger one fault at a time. Manifest errors cannot become missing
source, limit or root errors merely because execution inputs are also absent.

Every generator error returns this exact initialized JSON, with no prefix result,
truth refs, measured counts or partial commit list retained:

```json
{"repositories":[],"truth":[],"size":{"lines":0,"files":0,"repositories":0,"history":0},"excluded":[]}
```

This withholds partial results, not typed errors. `errors.As` must retain
`*benchenv.FleetError` or `*benchenv.FleetLimitError` with all prescribed fields.
No raw input bytes or host paths may be smuggled through `Error()`.

## Frozen baseline fixture, bytes and expected values

All future materialized roots, test control checkouts, fault sentinels and Git
metadata in this and subsequent sections stay in owned ignored workspace scratch.
Only seed literals/expected digests may be checked in. Test helpers must place
scratch under workspace explicitly, not use an ambient `t.TempDir()` base.
A test may use `t.TempDir()` only after its TMPDIR base is explicitly owned
workspace scratch and it verifies that resulting location. It never changes a
process-global environment during parallel tests. This docs lane does neither.

Call the following **chain-pair** manifest the baseline. The complete compact JSON
below is the input, not a generator receipt or a successful execution claim.
`sources["product"]` is the 29 literal bytes `package p\n// seed=@ENV_SEED@\n`.
`sources["key"]` is the exact tracked [fauxtown answer key](../../internal/connector/testdata/fauxtown/links.answer.json),
3,564 bytes including its final LF. Do not parse/re-marshal or regenerate that key.
Its SHA-256 is `6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51`.
The two product outputs are each the 20 bytes `package p\n// seed=7\n`.

```json
{"seed":7,"generator_version":"env-fleet/v1","template":"chain-pair","size":{"lines":4,"files":2,"repositories":2,"history":4},"repositories":[{"id":"a","head":"c1","commits":[{"id":"c0","parents":[],"files":[]},{"id":"c1","parents":["c0"],"files":[{"path":"p.go","content_id":"product","source_sha256":"f512a9dd67008c51b07b939347f088f5e981cd63bbf05c435408794b30fc266b","sha256":"6c8ba76a591c8167fbd155f911851fb2632d4d072067447ce57bce96775fb69b","mode":"100644","class":"product","seeded":true}]}]},{"id":"b","head":"c1","commits":[{"id":"c0","parents":[],"files":[]},{"id":"c1","parents":["c0"],"files":[{"path":"p.go","content_id":"product","source_sha256":"f512a9dd67008c51b07b939347f088f5e981cd63bbf05c435408794b30fc266b","sha256":"6c8ba76a591c8167fbd155f911851fb2632d4d072067447ce57bce96775fb69b","mode":"100644","class":"product","seeded":true}]}]}],"truth":[{"path":"links.answer.json","content_id":"key","source_sha256":"6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51","sha256":"6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51","mode":"100644","class":"generated","seeded":false}]}
```

Limits are exactly:

```json
{"max_repositories":2,"max_commits":4,"max_files":3,"max_bytes":3604}
```

The byte total is `2*20+3564=3604`, the entry total is `1+1+1=3`.
HEAD product size is `4 lines / 2 files / 2 repositories / 4 commits`.
The key is external truth, so every excluded HEAD category is zero even though
its own class is generated. `c0` timestamp is 946684800 and `c1` is 946684801
in EACH repository. Different repository IDs change commit messages and hashes.

| Byte source / seed | Bytes | SHA-256 |
|---|---|---|
| raw product | 29 | `f512a9dd67008c51b07b939347f088f5e981cd63bbf05c435408794b30fc266b` |
| expanded 0 | 20 | `32367435224e605587478fcee1fce49b7f9a8f55c833c72ea5f3503134eeb04b` |
| expanded 7 | 20 | `6c8ba76a591c8167fbd155f911851fb2632d4d072067447ce57bce96775fb69b` |
| expanded 8 | 20 | `3ccf9b4342d7e02f8cef254b4cffd2ffbeb93c673b67d8016746e28b6acf7d5c` |
| expanded uint64 maximum | 39 | `fc00ecbfa6c1d4d1168672234e50e620f89f9651632dd2522c8f245166761faa` |
| raw/copied key | 3564 | `6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51` |

| Seed | Repository | c0 | c1 / Head |
|---|---|---|---|
| 7 | a | `00bcf65b8f7c2a760ff78b15431923c2620fbf15` | `7610f835998aaeb2dce540bdfc392be7d248c375` |
| 7 | b | `c72bd20246d49d5b678805e564fa495317d8a8e1` | `ac0e50c13fee277da93c83e5f6274957fb10e8d0` |
| 8 | a | `00bcf65b8f7c2a760ff78b15431923c2620fbf15` | `111b9809b228845b20644b5f99c66d7cfdd8c115` |
| 8 | b | `c72bd20246d49d5b678805e564fa495317d8a8e1` | `44de61208fb4677bf59dadd527e0cc80f1ada8d1` |

| Seed | p.go blob | c1 tree |
|---|---|---|
| 7 | `07dc351ac0be30462ca61430272cf09c4f8ae4f9` | `ea7d8f429c8be0a645ba63b355d23f9fe967fb03` |
| 8 | `275240b875477f84fff97bff32478360622e9e86` | `5ade8aeda14ced85e11b9dc85b4f7ff73fc04e05` |

For seed 7 the canonical compact result JSON is EXACTLY the following, with Go
struct field order and no newline supplied by `json.Marshal`. The Markdown block's
final LF is presentation only. The error JSON above uses that same convention.

```json
{"repositories":[{"id":"a","head":"7610f835998aaeb2dce540bdfc392be7d248c375","commits":[{"id":"c0","commit":"00bcf65b8f7c2a760ff78b15431923c2620fbf15","parents":[]},{"id":"c1","commit":"7610f835998aaeb2dce540bdfc392be7d248c375","parents":["00bcf65b8f7c2a760ff78b15431923c2620fbf15"]}]},{"id":"b","head":"ac0e50c13fee277da93c83e5f6274957fb10e8d0","commits":[{"id":"c0","commit":"c72bd20246d49d5b678805e564fa495317d8a8e1","parents":[]},{"id":"c1","commit":"ac0e50c13fee277da93c83e5f6274957fb10e8d0","parents":["c72bd20246d49d5b678805e564fa495317d8a8e1"]}]}],"truth":[{"id":"key","path":"truth/links.answer.json","sha256":"6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51"}],"size":{"lines":4,"files":2,"repositories":2,"history":4},"excluded":[{"kind":"vendored","lines":0,"files":0},{"kind":"generated","lines":0,"files":0},{"kind":"test","lines":0,"files":0},{"kind":"build","lines":0,"files":0}]}
```

Repeat in two fresh roots, reverse repository/commit input order, and compare
this identical result and all checkout/truth bytes, modes, hashes and parent
edges. Do not compare compression or staging names. For seed 8 independently
update the product output digest. Only product checkout bytes, blob/tree and c1
hashes change. c0 hashes, key bytes/digest and size/caps remain identical.
The truth is declarative seed truth, not an independent oracle or admission.

## Additional frozen vectors

### Token, empty-byte and size vectors

Use the unchanged baseline truth key in every successful vector. For each vector,
explicitly supply raw/output digests from these literal byte strings, independent
of generator helpers. No omitted fixture field acquires a default.

For EACH row below, clone chain-pair, keep only repository a with empty c0 and
c1 parent c0/Head c1, and keep only p.go at c1. It is product/mode100644, with
ContentID `product` (`empty` for the Empty row). Supply its stated raw bytes under
that exact map key and retain sources[key]. Set Seeded as stated; use seed 7
unless a row explicitly supplies another seed. Replace both digests with SHA-256
of the literal raw/output bytes. Size is the row's product line count / 1 file /
1 repository / 2 commits. Caps are exactly 1/2/2/(3564+output byte length), not the
baseline's larger vector. Thus every byte case is an executable complete manifest
construction rather than an unspecified standalone expansion function.

| Vector | Exact raw bytes / expansion | Exact expectation |
|---|---|---|
| Multiple markers | `@ENV_SEED@:@ENV_SEED@\n`, seeded, seed 0 then uint64 maximum | `0:0\n` (4 bytes) then `18446744073709551615:18446744073709551615\n` (42 bytes), all markers replaced |
| Literal marker | same raw, unseeded, seed 7 then 8 | 22 unchanged raw bytes in both calls, no seed-derived difference for that file |
| Empty | present map key `empty` with zero-length bytes, unseeded | SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, zero lines and one file |
| Binary | bytes `00 ff 0a 42` in hex, unseeded | exact four bytes, two lines, no UTF-8/CRLF conversion |
| Unterminated/blank | `a\n\n` then `a` | respectively 2 and 1 lines, 3 and 1 bytes |
| CRLF | `a\r\n` | one line, 3 bytes, CR retained |

For the multiple-marker raw literal, `@ENV_SEED@` has 10 bytes, so raw length is
22. The 42-byte maximum expansion is counted after expansion, never from the raw
length. Pair every byte vector with exact SHA-256 and Git blob checks from the
literal bytes, rather than accepting equal results from two generator runs.

For HEAD-only size/exclusions use repository `a`, commits c0 then c1, Head c1.
c0 has only `old.go`, unseeded product bytes `old\n` (4 bytes). c1 has baseline
p.go plus these four unseeded files: `v.txt`, vendored, bytes `\n\n`;
`g.bin`, generated, bytes `00 78` hex; `t.txt`, test, empty bytes;
`b.txt`, build, bytes `b\r\n`. All modes are 100644 and ContentIDs are the
corresponding `old`, `product`, `v`, `g`, `t`, `b`; digests bind those exact bytes.
Truth is the baseline key. Result size is exactly `{lines:2,files:1,repositories:1,history:2}`.
Excluded, in order, is vendored 2 lines/1 file, generated 1/1, test 0/1, build 1/1.
Limits exactly 1 repository / 2 commits / 7 entries / 3,595 bytes:
`4+20+2+2+0+3+3564=3595`. old.go must be absent from checkout and HEAD counts.

For a deletion/full-snapshot vector, keep only repository a's baseline c0/c1 and
append c2 with parent c1 and empty files; Head c2. Size is 0/0/1/3, limits
1/3/2/3584, and no p.go remains at Head. All three commits remain reachable.
A second variant uses c2 with only `p.go/child`, unseeded product `Z\n`, proving
file-to-directory changes across snapshots are legal, with size 1/1/1/3, limits
1/3/3/3586. No inherited p.go can collide with that independent snapshot.

### Nested-tree and executable vector

Use only repository a, baseline empty c0, c1 parent c0 and Head c1. Replace c1
files by unseeded product `a.c` = `D\n`, `a/x` = `Z\n`, and executable `run` =
`#!/bin/sh\nexit 0\n` (mode 100755). Others use 100644, ContentIDs dot/nested/run.
Keep baseline truth. Size is 4/3/1/2 and exact caps are 1/2/4/3585.
The root tree entry order is **a.c, a/, run**, not plain basename a, a.c, run.
`a` is encoded mode 40000. The following Git identities bind byte order and modes:

| Object | Expected SHA-1 |
|---|---|
| a.c blob | `178481050188cf00d7d9cd5a11e43ab8fab9294f` |
| a/x blob | `e900b1c81c65dc52463027be827c1418fc7ff505` |
| run blob | `039e4d0069c5c26909f86c505b9de66182e6d1f3` |
| a tree | `6886685c813ca624f15179e887df8e941bea3483` |
| c1 root tree | `dad47a613c4b7368c409a30f761b1ecd5c8ec38a` |

Executable c1 is `8188ebaceb6c10ca2a97e0703349113f8270f4dd`. With only run's mode
changed to 100644, the tree is `23047c91ad17e18bb053029a59c9c2be97d4f1f6` and c1 is
`6052c6fc1658202126c51d2103434b4a3294c156`.

Reversing the three input files leaves these identical. Changing only run's
mode to 100644 preserves its blob bytes/hash and changes the tree/commit hash.
The checkout's executable bits must agree with the declared modes without relying
on ambient umask; require regular file versus executable, not a host mtime.

### Fork/join and parent-order vector

Use repository a with empty full snapshots and edges c0:[], c1:[c0], c2:[c0],
c3:[c1,c2], Head c3, plus unchanged baseline truth. Size is 0/0/1/4 and exact
caps are 1/4/1/3564. Canonical order/timestamps are c0/946684800,
c1/946684801, c2/946684802, c3/946684803 regardless of input permutation.
Every tree is the empty tree. Exact commit hashes are:

| Commit | Expected SHA-1 |
|---|---|
| c0 | `00bcf65b8f7c2a760ff78b15431923c2620fbf15` |
| c1 | `46060a9b7a113ee69767a62ea4d51e6ff2bea1ce` |
| c2 | `1e387ce2a8394816469374657296752881c2f810` |
| c3 | `b6af5df592ee41be790121f32e73abe923c8d37b` |

Swapping only c3's authored parents to [c2,c1] changes only its hash to
`101f492f713110334d242891823420f755d435cd` and preserves that ordered hash list in the
result. Re-sorting authored parents is a bug.

## Negative fixture and typed assertion table

Each row starts from chain-pair and changes ONLY the stated field unless the row
explicitly says otherwise. All `FleetError` assertions use `errors.As`, exact
Reason/Path and the exact initialized generator failure JSON. Limits assert the
other concrete error type and all four fields. Validation/refusal must precede
any staging creation when the problem is detectable from inputs/guard. Successful
schema validation is never mistaken for resolved bytes or an adequate environment.

Let `F=/repositories/0/commits/1/files/0` and `T=/truth/0`. These are textual
pointer abbreviations only. Expand them literally for every assertion.

| Mutation | Typed reason | Exact pointer |
|---|---|---|
| GeneratorVersion missing/null/empty/unknown/non-string, Template whitespace-only | FleetShape | `/generator_version`, `/template` respectively |
| Invalid repository/commit/content ID, each independently: empty, uppercase, underscore, leading digit | FleetShape | `/repositories/0/id`, `/repositories/0/commits/1/id`, `F/content_id` respectively |
| File mode 120000 or 160000, class unknown | FleetShape | `F/mode`, `F/class` respectively |
| Nil repositories, truth, commits, files or parents, each independently | FleetShape | exact pointer of that nil collection |
| Negative size component, each of lines/files/repositories/history | FleetShape | `/size/<component>` |
| Malformed digest, independently source and output, every file/truth instance: 63/65 characters, uppercase, nonhex | FleetShape | pointer of that mutated digest |
| p.go path -> `../p.go`, `/p.go`, `a//p.go`, `.git/config`, `x/.git/config`, `a/../p.go`, `./p.go`, `p.go/`, `a\\p.go`, `C:p.go`, each ASCII control byte U+0000..U+001F or U+007F in the path independently | FleetPath | `F/path` |
| Truth path -> each same invalid path | FleetPath | `T/path` |
| Append repository a | FleetDuplicate | `/repositories/2/id` |
| Append c1 in repository a | FleetDuplicate | `/repositories/0/commits/2/id` |
| Append p.go in repository a/c1 | FleetDuplicate | `/repositories/0/commits/1/files/1/path` |
| Append baseline truth path again | FleetDuplicate | `/truth/1/path` |
| c1 parent -> missing | FleetParent | `/repositories/0/commits/1/parents/0` |
| c0 parents -> [c1], forming cycle | FleetCycle | `/repositories/0/commits/0/parents/0` |
| c1 parents -> [c1], self-cycle | FleetCycle | `/repositories/0/commits/1/parents/0` |
| c1 parents -> [c0,c0] | FleetDuplicate | `/repositories/0/commits/1/parents/1` |
| Head -> missing or c0 (c1 unreachable), independently | FleetParent | `/repositories/0/head` |
| Add p.go/child beside p.go | FleetPath | `/repositories/0/commits/1/files/1/path` |
| Add links.answer.json/child beside truth key | FleetPath | `/truth/1/path` |
| Delete sources[product], supply nil sources, or set F.ContentID to unknown without adding raw bytes | FleetDigest | `F/source_sha256` |
| Delete sources[key], or change T.ContentID to unknown without bytes | FleetDigest | `T/source_sha256` |
| Change raw product bytes without changing source digest | FleetDigest | `F/source_sha256` |
| Change raw key bytes without changing its source digest | FleetDigest | `T/source_sha256` |
| Expected product output digest -> 64 zeroes | FleetDigest | `F/sha256` |
| Expected truth output digest -> 64 zeroes | FleetDigest | `T/sha256` |
| product source -> `package p\n`, source/output digests updated to that literal, Seeded remains true | FleetShape | `F/seeded` |
| Each declared size component +1 independently, valid raw bytes/digests retained | FleetSizeMismatch | `/size/lines`, `/size/files`, `/size/repositories`, `/size/history` respectively |
| Zero or negative cap, each independently | FleetShape | `/limits/max_<kind>` |
| Accounting int64 overflow, via bounded private arithmetic fixture, no huge allocation | FleetShape | `/size` |
| Invalid workspace/ignore/root, existing/dangling symlink target, missing/non-directory/symlink parent or publication collision | FleetTarget | `/root` |
| Injected staging/object/ref/checkout/truth/chmod/rename I/O failure | FleetIO | `/root` |
| Cancelled/deadline-expired valid call, before or during writes or before publication | FleetCancelled | `/root` |

Raw JSON/CUE syntax, missing required boolean/numeric members, fractional or
out-of-range numbers, unknown members and seed outside uint64 cannot be represented
by this typed leaf signature. They remain the unchanged core's
`TestValidateFleetManifest` assertions, not a new source-decoding overload or a
claim that a Go uint64 can hold an invalid number. The implementer does not edit
or weaken those core tests.

For exact one-short cap cases, use these values. Each test changes only its cap.

| Kind | Limit | Observed | Path |
|---|---|---|---|
| LimitRepositories | 1 | 2 | `/limits/max_repositories` |
| LimitCommits | 3 | 4 | `/limits/max_commits` |
| LimitFiles | 2 | 3 | `/limits/max_files` |
| LimitBytes | 3603 | 3604 | `/limits/max_bytes` |

With all four caps short, repositories wins. With only commits/files/bytes short,
commits wins, then files when only files/bytes are short. Equality at every cap
must succeed. Add the product entry to c0 in BOTH baseline repositories while
retaining c1 unchanged: HEAD size remains 4/2/2/4, total entries become 5 and
bytes 3644. MaxFiles=4 refuses observed 5, MaxBytes=3643 refuses observed 3644,
and 2/4/5/3644 succeeds. Reused bytes/blobs never escape snapshot accounting.

For competing errors, start with the duplicate-file mutation at input index 1
and invalid Head at `/repositories/0/head`. Duplicate's pointer sorts first and
wins. Reverse only the commit array: its pointer now uses original commit index
0 and still wins. Add an unknown-version shape error: `/generator_version`
wins before semantic validation. Separately combine wrong output digest with all
caps short and an invalid root: the digest wins before caps/root. Missing bytes
never becomes a filesystem error or a compiler/template-registry lookup.

Schema's pinned `TestFleetManifest_GeneratorVersion` asserts the named
GeneratorVersion field, GeneratorVersionV1 exactly "env-fleet/v1", unchanged JSON
key and missing/null/empty/unknown/non-string refusal at /generator_version with
initialized failure JSON. Its SeedInput counterpart refuses ModeShape at
/seed/manifest/generator_version. Schema owns those tests and #GeneratorVersion;
ENV-3 neither copies nor implements the schema. Generator tests use that named
constant and retain the same wire value and exact fixture bytes.

## Complete implementing test inventory (W3-2)

Every test belongs to `internal/benchfleet/generate_test.go`, has the exact prefix
selected by ENV-3's proof command, and uses only owned ignored workspace scratch.
These are implementing obligations, not tests claimed to exist in this draft.
A table row may be implemented as enumerated subtests, but none of its assertions
may be dropped. A weaker test is a FIX under W3-2.

| Exact test name | Frozen assertions |
|---|---|
| `TestBenchmarkFleet` | Materialize chain-pair twice in fresh roots. Compare ALL checkout/truth bytes and regular/executable modes, raw object identities and ordered parent edges, refs/HEAD, the exact seed-7 result JSON and literal hashes above. Assert key bytes/digest, Head reachability and timestamps separately. Seed 8 with its independently updated digest gives the exact seed-8 hashes/bytes, unchanged c0/truth/size, and no false independent-environment claim. No ingest/native checker runs. |
| `TestBenchmarkFleet_InvalidManifest` | EACH representable shape/path/duplicate/parent/cycle/Head mutation above reaches the generator's revalidation and returns the exact core typed error/pointer and initialized generator failure JSON. Root and staging remain absent. Schema-valid chain-pair without source bytes remains accepted by the pure validator and is refused only by the generator's source stage. Preserve malformed-source/number/missing-member tests in their unchanged core owner rather than reassigning them to this typed API. |
| `TestBenchmarkFleet_SourceBytes` | EACH raw/digest/missing/unknown lookup and missing-marker mutation above returns its exact reason/pointer before writes. Present nil and zero-byte source values both mean available empty bytes and differ from an absent key. ALL multiple-marker, false-Seeded, zero/max-uint64, empty, non-UTF8, blank/unterminated and CRLF vectors preserve exact literal bytes, lengths, line counts, SHA-256 and Git blob hashes. A valid but unreferenced `sources["unused"]` with arbitrary bytes has no output/count/result effect. No disk/template fallback. |
| `TestBenchmarkFleet_CanonicalDAG` | Fork/join vector yields exact c0..c3 hashes, timestamps, authored parent hashes and lexical-ready order above. Exercise all 24 input commit permutations and reversed repository ordering for baseline. Swap only merge parent order and assert the stated changed c3 hash with other hashes unchanged. Valid IDs reused across different repositories stay separate. Missing/self/cyclic/unreachable cases remain typed refusals, never successful truncation of history. |
| `TestBenchmarkFleet_FullSnapshots` | The empty-c2 deletion vector has no p.go at Head, size 0/0/1/3 and exact snapshot entry/byte caps 1/3/2/3584. All parent commits remain intact. The file-to-directory variant materializes p.go/child as exactly `Z\n`, has size 1/1/1/3 and caps 1/3/3/3586, proving no inherited-file overlay. No fixture source changes. |
| `TestBenchmarkFleet_TreesAndModes` | Nested/executable vector has exact object identities, directory mode 40000 and Git ordering a.c, a/, run, size 4/3/1/2, caps 1/2/4/3585. Reverse files and obtain identical bytes/result. Read raw tree bytes independently and verify 20-byte OIDs, not textual hex entries. Change only run mode to 100644: blob unchanged, exact regular-mode tree/commit hashes above and checkout executable bits removed. No ambient umask may turn declared mode 100755 into non-executable output. |
| `TestBenchmarkFleet_SizeAndExclusions` | HEAD-only old.go/exclusion vector yields exact size 2/1/1/2, excluded [{vendored,2,1},{generated,1,1},{test,0,1},{build,1,1}], and caps 1/2/7/3595. old.go absent from Head but present in c0, truth outside every repo/count. EACH +1 declared-size mutation returns exact FleetSizeMismatch pointer and no writes. All excluded zero rows remain in baseline result. No scale or adequacy inference from padding. |
| `TestBenchmarkFleet_Limits` | Equality and EACH one-short/zero/negative cap above, exact concrete error fields and ordering across simultaneous exhausted caps. Repeated-snapshot vector counts all 5 entries/3644 bytes despite shared content, equality succeeds, one-short refuses. Every input refusal leaves root/staging absent and all caller sentinels unchanged. No clamp, implicit default, retry or partial result. |
| `TestBenchmarkFleet_Overflow` | Exercise the SAME checked accumulation/expanded-length helpers the entry point uses: MaxInt64+0 succeeds exactly, MaxInt64+1 and multiplication beyond MaxInt64 refuse as FleetShape at /size before allocation/writes. Expansion count at equality is representable; one-over cannot wrap to an accepted small number. Use synthetic integer operands through package-private seams, never allocate exabytes or bypass the entry point's checked arithmetic. The failure adapter returns the exact empty result JSON. |
| `TestBenchmarkFleet_WorkspaceRoot` | Positive control checkout has a real workspace and root `.gitignore` /workspace/ exclusion, existing owned parent and absent child root. A regular `.git` worktree marker also works. With only the nested control marker reported absent by a package-private per-call guard-read seam and all enclosing reads unchanged, assert successful chain-pair generation through the qualifying host ancestor with its real workspace and ignore rule. For a separate missing-checkout subtest, enumerate the finite ancestor chain from the existing requested root's parent through the native filesystem root inclusive. Use that private per-call seam to report not-exist for `<ancestor>/.git` at EVERY enumerated ancestor, including control and host markers, delegating all other reads unchanged. Record that every enumerated marker path was checked and none qualified. Assert FleetTarget /root, the exact initialized failure JSON, zero staging writes and no partial success. Do not delete or modify any real marker or host/global state. Separately remove workspace or ignore rule, add a later negation, make ignore unreadable through the fault seam, use workspace itself, source path, workspace-other prefix, an outside-owned sentinel path, empty root, traversal, existing file/empty directory/nonempty directory/dangling symlink, missing parent, parent file, parent or workspace symlink (inside and outside targets). EACH returns FleetTarget /root before staging. All paths/sentinels used for negative tests remain physically inside the host's owned ignored workspace; nothing touches a real external directory. Parent/control/ignore/key bytes stay unchanged. |
| `TestBenchmarkFleet_PathCollisions` | Duplicate and file/directory-prefix rows for repository snapshots AND truth return their exact pointers. Nested `.git` paths refuse before writes. Inject filesystem aliasing for two otherwise valid paths, and for a path aliasing Git metadata, at the owned filesystem seam: FleetTarget /root, cleanup and no result. This proof does not depend on whether the test host has a case-sensitive filesystem. Never widen a pure-schema refusal to patch core policy. |
| `TestBenchmarkFleet_PublicationCollision` | At the deterministic before-publication seam, create a caller-owned empty target, then a target with a sentinel in a separate subtest. Also replace the owned parent with a symlink inside the scratch control at that seam. Each must return FleetTarget /root, preserve the colliding caller target/sentinel or symlink without following it, remove only owned staging/reservation, return exact failure JSON and expose no successful prefix. No overwrite-on-rename or arbitrary parent cleanup. |
| `TestBenchmarkFleet_Cancelled` | Valid baseline with pre-cancelled context and an explicitly expired fixed deadline returns FleetCancelled /root with zero writes. A deterministic callback cancels after the first owned write; another cancels immediately before publication. Both remove owned staging/reservation, leave no result root, preserve caller parent/key/siblings, retain the exact typed error and initialized failure JSON. No sleeps or real-clock deadline race. |
| `TestBenchmarkFleet_IO` | Inject an error, one at a time, at staging create, object write, ref write, checkout write, truth write, chmod and final rename. Each yields FleetIO /root and exact empty result JSON. For failures after staging exists, only owned staging/reservation is removed. Caller preexisting roots/parent/keys/sibling sentinels survive byte-for-byte. Root/containment/target collisions are FleetTarget instead. Fault helpers do not fail cleanup itself or alter shared permissions, so their cleanup assertion is meaningful. |
| `TestBenchmarkFleet_JSONArrays` | Marshal baseline and all failed results. Exact baseline/failure literals match. Repositories/Commits/Parents/Truth/Excluded are nonnil wherever present, root Parents encode [], never null, and four success excluded categories remain ordered. Every result field has its unchanged lower snake case key. No root, sources, raw bytes, admission/pass status or new field is serialized. Use errors.As on every refusal and assert the prescribed error fields survive. |
| `TestBenchmarkFleet_InputConservation` | Deep-snapshot manifest slices/parent order, source-map keys/bytes, limits and complete original fauxtown inventory before the first call. Successful seed-7/seed-8/permuted calls and EACH negative/fault leave caller inputs and all original fixture bytes/modes unchanged. The implementation may normalize private copies only. Compare source inventory against independent read bytes, not generator output. Test cleanup owns only its explicit scratch. |
| `TestBenchmarkFleet_IndependentBoundary` | Parse non-test imports/calls: implementation imports only standard library and schema/benchenv, no checker, command package, costtest, store, client or other oracle. Materialization cannot call a subprocess/network/compiler/ingest or fingerprint hasher. Tests similarly import no harness/checker/CLI. All fixture output/control/staging paths are asserted below owned ignored workspace and no generated checkout/Git object/truth output is added to the source index. Conservation and capability claims are separate from generator success. |

Package-private filesystem/cancellation seams may replace owned operations for
these tests. They add no exported DTO, callback, option or future policy field.
No process-global hook, environment variable, clock override or shared path is a
test seam. A manifest fault must not be disguised as an injected write error.
The independent object oracle must build raw headers/tree/commit payloads from
literals in the test, without calling generator encoding/hashing helpers.

The implementer must first run a real semantic failing test under W3-2. One
acceptable RED is the baseline cap test with MaxFiles=2: a temporary functional
implementation that counts only checkout files incorrectly succeeds instead of
returning LimitFiles/2/3. Another is raw tree-order or full-snapshot deletion with
the exact wrong behavior witnessed. Merely compiling against a missing package,
missing symbol or missing method does not establish RED. Preserve the actual
semantic failure output before GREEN, then run the whole named inventory.

## Proof boundary, publication and upstream wiring

The future implementing lane's bounded acceptance command, not executed here, is:

```sh
go test -race ./internal/benchfleet -run '^TestBenchmarkFleet'
```

Its package work uses the lane's released capacity/lease rules. The controller owns
the full gate. `fathom verify --service fathom --plan docs/plans/bench-environments.md`
must prove a claim before anyone checks ENV-3. This documentation does not claim
Go API compilation, semantic RED/GREEN, race, runtime/native correctness, gate,
resource timing, concept coverage, independence, admission or plan completion.

Author documentation proof is limited to exact declaration/signature/CUE
conservation against the amended core (historical pins retained as provenance), unchanged fauxtown bytes,
Markdown fences/local citation targets, no held commit ancestry, and independent
literal byte/count/Git-object arithmetic for these vectors. Git object arithmetic
without a checkout is not materialization or a native benchmark. Local evidence
lives only under workspace, including the source hashes and expected arithmetic.
Separate critic must review this distinct file, assertions and T6 conditions before
W2-3 approval/publication handoff. The author does not self-award LAND.

Upstream wiring requests, without shared-file edits in this unit:

* Publish/integrate the exact approved core after its independent publication
  review, supplying its conserved fleet schema API. Other authored ENV records
  retain REC ownership; concept struct keys use schema/cue vet and pathcheck only.
* Integrate this separate reviewed contract and grant a fresh implementation lane
  only after its own approval. The held full ENV contracts are not stacked or
  required to provide excluded checker/CLI behavior.
* ENV-C5 later owns `runEnvironmentSeed`, native receipt consumption and joined
  negative-twin/coverage proofs. ENV-G1 to ENV-G7 later own presets. BEN-9 retains
  the sole fingerprint hasher. None of those is implemented or proof-claimed here.

## Decision models

None applies. The materializer performs deterministic byte, path, DAG and bound
operations with provider `none`. It invokes no model, judges no truth and awards
no admission, so there is no judged site or additional opt-out mechanism.
