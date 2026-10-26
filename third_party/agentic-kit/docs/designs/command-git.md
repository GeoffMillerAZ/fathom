# Controlled Git reads

Status: implementation specification, 2026-10-24. Authority: [0040](../intent/decisions/0040-git-reads-require-registered-metadata.md).
Scope: GT6's three typed read recipes inside `command-guard`. This is not a Git sandbox,
authorization service, command rewriting layer or hook installer.

```mermaid
flowchart LR
  R[Registered protected metadata] --> V[Verify binary/config/topology]
  V --> G[Fixed status / object diff / history]
  G --> B[Bound and validate selected data]
  B --> D[Full DLP and required scanner]
  D --> P[Explicit bounded source-data view]
```

## GT-G1: registration and executable

`git-inspect --root ABS --scope FILE --policy FILE --request FILE --format json|text`
uses the existing strict 1 MiB/depth-24 controls. Scope optionally declares exactly
`requiredGitProfiles: ["git-read/1"]`, matching the optional policy `git` object.
Both are required for Git inspection. Legacy file inspection remains unchanged.
Every required layer must allow the operation; existing minima, denied prefixes,
required regex inventory and scanner requirements still apply.

Policy `git` contains `profile`, trusted absolute `executable`, SHA-256,
`expectedVersion` (exact bounded `git version ...` string), `timeoutMillis` <=30000,
and `repository`. Registration contains `kind: worktree|bare`, canonical `root`,
`gitDir`, `commonDir`, common `configSHA256`, `worktreeConfigSHA256` (hash or `absent`),
and `commonDirFileSHA256` (hash or `absent`). Index and object locations are fixed as
`gitDir/index` and `commonDir/objects`; no caller index/object override. Root must
match the CLI target. A bare registration uses root/gitDir/commonDir equal.

The operator must protect metadata, directory entries/resolution chains, config and
the executable outside candidate write authority for the duration of the invocation.
The wrapper validates registration consistency; it does not enforce that OS boundary.
Use explicit admin/common/worktree/object/index context, never discover through an
untrusted worktree `.git` file. Linked worktrees are supported by their separately
registered admin directory and common store, not inferred from candidate content.

Before repository commands, verify bounded regular control files and their hashes,
reject symlinked metadata, unregistered `commondir`, object alternates, grafts, shallow,
promisor and unsupported extension state. Common config <=1 MiB, worktree config
<=1 MiB, index <=16 MiB, at most 65,536 metadata entries per validation sweep.
This explicit pilot ceiling can hold a larger healthy repository; it is not a
repository-health verdict or a scalable inventory/cache implementation. Parse copied config
using fixed `git config --file COPY --no-includes --null --list` in the private
context; reject includes and unqualified executable-bearing configuration. No home,
system/global config or candidate config is allowed to define this command's authority.
Unknown configuration that affects execution or repository layout holds.

Snapshot the pinned executable into the guard's private preparation directory and
check its exact version. Reuse guard preparation/transport narrowly with per-call
injection and a shared parent deadline; no mutable global test seam. Private copies
and HOME/TMP cleanup have the same failure limits as GT5a. No report files.

## GT-G2: fixed execution and resource contract

The command builds argv internally. No raw argv, shell, arbitrary refs, caller index,
remote URL, plugin, pager, mailmap, pretty format or executable recipe is accepted.
All OIDs are full lowercase 40/64 hex, with length matching storage format. `HEAD`
is the only named ref choice and each distinct ref is resolved once per invocation
to a full commit before use. A HEAD-to-HEAD comparison uses the same pinned commit.

Use a minimal environment, explicit context and private empty helper/hook directories.
Disable system/global attributes/config, replacements, lazy fetch, optional locks,
prompts, fsmonitor, pager, external diff/textconv, rename heuristics, automatic GC/
maintenance, signature verification and all network protocols. Fixed calls include
`version`, protected config parsing, object-format/ref/type/size queries,
`status --porcelain=v2 -z --untracked-files=all --ignore-submodules=all --no-renames`,
raw recursive `diff-tree -z`, unfiltered `cat-file`, and bounded `rev-list`.
Only validated OIDs, literal selected paths and bounded numbers enter arguments.

One invocation deadline covers preparation, every Git call, scanning and projection.
Stdout <=16 MiB per metadata call, stderr <=64 KiB, complete selected payload <=64 MiB,
each blob/commit <=4 MiB, <=32 selected changes/commits and <=1000 returned rows;
effective existing policy limits may tighten these. Excess, timeout, incomplete pipe,
malformed output, unsupported source or cleanup failure withholds all source data.
Private process observations distinguish attempted/started/completed calls. A valid
output budget covers the entire report including errors; if no fixed diagnostic can
fit, exit 2 writes no bytes. Invalid controls cannot supply an authoritative budget.
Process
confinement, hard memory bounds and hostile-filesystem wall time are not promised.

## GT-G3: typed operations

Request version is `command-guard.git-request/1`. Fields are exact by kind, including
presence checks on forbidden zero-valued fields. `paths` is 1..32 canonical portable
literal path selectors (`.` means the whole registered tree); never Git magic/globs.
Paths outside the selection are excluded without publishing their names/counts.

| Kind and fields | Result and proof |
|---|---|
| `git-status`, `paths` | Selected porcelain-v2 entries with explicit ignored/submodule exclusion and stable closed XY/type states; never infer clean/absent from an omitted entry. Bare status is unsupported. A qualified silent missing-ref probe supports unborn status. HEAD/index/config fingerprints before/after detect observed drift; result is always a live non-atomic observation. |
| `git-object-diff`, `base`, `target`, `paths` | Resolve exact commits/trees, parse complete bounded raw NUL-delimited change records, require every returned path to match the literal scope, read full regular before/after blobs after size/type checks and verify object hashes. Report the resolved base/target commit IDs. Return bounded structured before/after line views, not a claimed unified patch. Reject symlink/gitlink/binary or unsupported modes. |
| `git-history`, `target`, `maxCommits` (1..32) | Report the resolved target commit. Fixed traversal reads at most N+1 IDs, then full selected raw commit records; N+1 means older history remains unexamined, not a known omitted total. No log signature verification, notes or mailmap. Validate commit headers/parents and encoding, with bounded parent count and full object hash checks. Bounded standard signature/mergetag continuations are scanned as raw source, never signature or attribution proof. |

History and diff work on bare registrations as well. Status conflict records need a
defined closed representation or must hold as unsupported. A status success describes
validly disclosed Git observations, not repository health, clean state or atomicity.
Pre/post checks cannot detect an intervening change and restoration. Missing objects,
partial-clone state or changing required context remains incomplete, not fetched.

## GT-G4: disclosure and output

Capture all selected data before rendering. Scan complete selected before/after blobs
and commit records, including bytes omitted from the view. Scan the exact normalized
source-data representation too: NUL-delimited Git wire records never go straight to
a terminal. Invalid UTF-8/control/path syntax holds. Apply existing DLP and every
required optional scanner; a finding blocks and unknown/incomplete checks hold.
Selectors and resolved commit identities are part of that scanned representation,
even if no rows match. Blocked/incomplete reports contain no source selectors,
commit identities or rows. Operation is a closed discriminator in the CUE/Go reports.

Each blob side presents only a contiguous prefix of at most eight lines, each at
most 512 bytes. The first oversized/omitted line stops the visible prefix; later
lines are counted as omitted rather than falsely presented as adjacent. Commit
headers are at most 8 KiB; a message over 4 KiB is omitted entirely with its byte
count. All omitted raw content remains subject to full checks. Whole rows can also
be omitted to satisfy output/result limits, with explicit counts.

This profile explicitly discloses selected Git source IDs, paths and commit metadata
as source data after checks. Private executable/config paths/hashes and error bodies
remain absent from diagnostics. Metadata disclosure needs the receiver's authority;
IDs and hashes are not permission, proof of ownership, authorship or confidentiality.

Versioned JSON/text output uses escaped source-data rows, scan status, fixed error
codes, invocation counts, exact selected scope, observation/coherence limits and
explicit projection/older-history incompleteness. Existing 0/1/2 exits and
`noActionAuthority: true` remain. No content before every check finishes; sink failures
can leave already-checked partial output and cannot be retried blindly.

## GT-G5: qualification

Hermetic tests use injected runners/test processes, never installed Git/network.
Native qualification uses only owned synthetic repositories and a trusted Git build.
Cover ordinary, linked and bare context, SHA-1/256 where supported, exact full IDs,
leading-dash/space paths, malicious gitfile/commondir, includes/alternates/filters/
fsmonitor, replacement/lazy-fetch state, ignored/submodule and unmerged semantics,
NUL/terminal/binary/mode/size errors, unexpected refs/output, late source findings,
HEAD/index/config drift, shared deadline and pipe limits. Fingerprint fixture files
before/after and use planted helper/network witnesses for the claimed paths. A quiet
witness is scoped evidence, never proof of OS isolation. Record source/Git/platform
identities, retained failures, actual setup/latency observations and independent review.
GT4 still owns other native platform qualification; GT16 owns mutation/publication.
