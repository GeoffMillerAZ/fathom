# command-guard (experimental)

## Qualify a native host

Build the guard and the separate [`command-qualify`](../command-qualify/README.md)
driver, select their trusted byte pins, and run the fixed file profile on the host
where it will be used. The optional scanner profile also requires its trusted exact
Gitleaks binary/pin. Receipts disclose actual OS/architecture and every required
case; missing capabilities remain unknown. No installation or hooks are needed.
These observations cover their fixed synthetic file scope, not native Git, arbitrary
formats, concurrent hostile writers, whole-process isolation or production safety.

## Optional contextual concern check

For an optional contextual concern check, see the
[classifier guide](../../docs/command-classifiers.md). The experimental profile
requires an explicit `-tags command_classifier` build, independent public/synthetic
input and raw policy pins, runs after hard
checks and holds all rows on concern/uncertainty/failure. Its loopback hop is not
local-inference or model-quality proof. The native fixture profile performs no AI
inference. No model, provider, hook or global configuration is installed or enabled.
The ordinary build excludes the client and rejects selected classifier controls
before source/scanner work. `help` reports the compiled capability. Pin and qualify
the two artifacts separately; the experimental artifact has additional startup cost.

## Optional repeated-input scan reuse

After upgrading the guard and its CUE/report decoder together, the existing pinned
Gitleaks scanner policy may add `"reuseEqualInputs": true`. Keep its independently
required scanner ID, executable/hash/version profile, limits and regex rules.
Omit this field for the existing behavior; it defaults to false. It does not select
a different scanner or turn off a check.

The enabled profile reuses a fully verified zero-finding observation only for exact
equal input bytes within this invocation. Every new invocation revalidates controls,
snapshots, executable and version. Findings, errors, skipped input and partial scans
are not reusable. Cache capacity is 8 MiB/256 private copied inputs; full capacity
means an ordinary scan. Deadlines, cleanup failures and later findings still hold
all selected output. There is no disk cache or stored authorization.

Enabled receipts add `externalScanner.reuse`:

```json
{"profile": "invocation-exact/1", "files": 1, "bytes": 1024}
```

Attempted/invoked/completed and
submitted/observed-byte counters count actual subprocess work. For three logical
inputs A/B/A, a successful reuse example has two completed scans plus one reused
input. Use three distinct selected paths containing A/B/A bytes; repeating the same
path is an invalid request. Require successful final status and `completedFiles + reuse.files == expectedFiles`;
never pretend the reused input started a process. Version probes remain separate.
For Git, the legacy files counters count captured scanner-input units, including
metadata/representation, rather than only filesystem files. Treat an unsupported
receipt profile as unknown. Smaller process counts alone do not prove task savings.
These counters reveal aggregate equality/size information about selected inputs;
enable reuse only where that metadata is permitted.
See [OR's exact contract and qualification bars](../../docs/designs/command-observation-reuse.md).

## Inspection views

Both `inspect` and `git-inspect` accept `--format compact`. It chooses escaped JSON
tuples only when they are shorter than the ordinary minified report. Small reports,
unsupported future row shapes and all failures retain ordinary JSON. Select the
decoder by `version`, never by the requested flag. Existing `json` and `text` remain
available. Every selected source is still checked before disclosure.

For a complete owned file trial from the repo root:

```sh
go build -o /tmp/command-guard-demo ./cmd/command-guard
/tmp/command-guard-demo inspect --root "$PWD/cmd/command-guard/testdata" --scope cmd/command-guard/testdata/scope.json --policy cmd/command-guard/testdata/policy.json --request cmd/command-guard/testdata/request-read.json --format compact
```

Inspection version `command-guard.compact-report/1` carries
`rowFormat: "file-line-text/1"`: a row `[0, 7, "example"]` means the original request's
first file, line 7, text `example`. Git version `command-guard.compact-git-report/1`
uses `status/1`, `object-diff/1` or `history/1`; the exact positional layouts are in
[CV-2](../../docs/designs/command-compact.md#cv-2-frozen-tuple-grammar). All header,
scanner, source, omission and live-observation fields retain their meanings. Tuples
are data, never commands. Keep pins and controls for detail requests; rerun the guard.
Existing Git view ceilings and freshness rules still apply. This adds no raw bypass,
observation cache, measured token count or claim of lower production review cost.
When an accepted effective output cap cannot fit even a fixed failure report with
its newline, exit 2 writes no bytes. A partial writer may already have delivered a
verified prefix and must not be blindly retried.

`git-inspect` is the registered Git read profile. It has three typed requests:
`git-status` with `paths`, `git-object-diff` with `base`, `target`, and `paths`, and
`git-history` with `target` and `maxCommits`. Each request uses version
`command-guard.git-request/1`; `HEAD` or a full lowercase object ID is accepted
where an object is required. Paths are portable literal selectors, with `.`
selecting the registered tree. No caller Git flags or raw command arguments are
accepted. Run it with the same control-file flags as `inspect`:

```sh
command-guard git-inspect --root /registered/tree --scope /protected/scope.json --policy /protected/policy.json --request /protected/request.json --format json
```

The protected scope must declare `requiredGitProfiles: ["git-read/1"]`; the policy
must contain a matching `git` object. For example, the policy object has
`profile`, an absolute `executable`, its lowercase `sha256`, the exact
`expectedVersion` string, `timeoutMillis` (1–30000), and a `repository` object:

```json
{"profile":"git-read/1","executable":"/protected/git","sha256":"<64 lowercase hex>","expectedVersion":"git version 2.54.0","timeoutMillis":30000,"repository":{"kind":"worktree","root":"/registered/tree","gitDir":"/registered/tree/.git","commonDir":"/registered/tree/.git","configSHA256":"<64 lowercase hex>","worktreeConfigSHA256":"absent","commonDirFileSHA256":"absent"}}
```

For a first **owned synthetic ordinary repository** trial, create three sibling
control files outside that repository. Keep the protected registration independent
of candidate-supplied `.git` discovery. The complete `scope.json` is:

```json
{"version":"command-guard.scope/1","requiredLayerIds":["trial"],"requiredRuleIds":["trial-marker"],"requiredGitProfiles":["git-read/1"]}
```

The complete `policy.json` below is a template. Replace all absolute paths and both
hash placeholders with independently selected physical paths and actual lowercase
SHA-256 values. The root must also match `--root`. This ordinary-repository example
uses the same admin/common directory; do not copy that topology for linked/bare use.

```json
{
  "version": "command-guard.policy/1",
  "layers": [{"id": "trial", "allowedOperations": ["git-status", "git-object-diff", "git-history"], "limits": {"maxFileBytes": 4194304, "maxTotalBytes": 67108864, "maxOutputBytes": 65536, "maxResults": 32}, "deniedPathPrefixes": []}],
  "rules": [{"id": "trial-marker", "pattern": "SECRET"}],
  "git": {"profile": "git-read/1", "executable": "/protected/git", "sha256": "<actual executable SHA-256>", "expectedVersion": "git version 2.54.0", "timeoutMillis": 30000, "repository": {"kind": "worktree", "root": "/registered/tree", "gitDir": "/registered/tree/.git", "commonDir": "/registered/tree/.git", "configSHA256": "<actual config SHA-256>", "worktreeConfigSHA256": "absent", "commonDirFileSHA256": "absent"}}
}
```

On the qualified Mac host, `shasum -a 256 /protected/git` and
`shasum -a 256 /registered/tree/.git/config` collect these byte identities. Run the
independently trusted executable's `--version` from a trusted directory and put its
exact single-line output in `expectedVersion`. A digest pins bytes, not provenance or
OS protection. Executable symlinks and symlinked path ancestors must be resolved in
the operator's registration. The command never downloads or discovers a Git binary.

Start with this complete `request.json`, then use the invocation above:

```json
{"version":"command-guard.git-request/1","kind":"git-status","paths":["."]}
```

After the synthetic repo has commits, history uses
`{"version":"command-guard.git-request/1","kind":"git-history","target":"HEAD","maxCommits":1}`.
Diff uses
`{"version":"command-guard.git-request/1","kind":"git-object-diff","base":"<full base commit OID>","target":"HEAD","paths":["."]}`.
Use `help` or `git-inspect --help` without any control files. The trial regex detects
only its literal marker; production rule/scanner scope needs its own qualification.

For a linked worktree, register its admin `gitDir`, shared `commonDir`, and the
`commondir` control-file hash. For a bare repository, `root`, `gitDir`, and
`commonDir` are the same; status is unsupported. The operator must protect this
registration, metadata, directory resolution chains, config, and executable from
candidate writes for the invocation. A local JSON policy cannot enforce that OS
boundary. The command verifies bounded metadata and copies the pinned executable
to private preparation space. It runs fixed Git reads with a minimal environment,
applies every required policy layer and scanner, and emits source rows only after
all checks finish. Git IDs, paths, and commit metadata in successful rows are
source-data disclosure; the receiver needs authority to receive them.
Metadata inventory is capped at 65,536 directory entries per validation sweep;
larger repositories hold as incomplete. This pilot ceiling is not a memory bound.
Selected status and diff observations are capped at 32 rows before projection;
larger selections hold with `result_limit`. Narrow the explicit path selection
within the same required controls. Compact output does not raise source limits.

Exit 0 means valid selected observations with no configured finding, 1 blocks
disclosure, and 2 means invalid or incomplete evidence. Status always describes
a live, non-atomic worktree observation; an unreported path is not clean or absent.
Ignored paths and submodule walks are excluded. Diff returns regular text blob
before/after line views, not a patch; symlinks, gitlinks, binary files, conflicts,
and unsupported modes hold. History's `hasMore` means Git reported another valid
commit ID beyond the selected bound; its content remains unexamined and no omitted
total is claimed. `omittedLines` and
`projectionTruncated` describe view limits. The guard does not fetch, mutate,
install Git, attest repository ownership, or prove the absence of sensitive data.
Metadata validation is capped at 65,536 entries per sweep; larger otherwise healthy
repositories hold. The deadline cancels child processes and checks preparation
cooperatively, but cannot interrupt a blocked filesystem operation. Only the tested
native host is qualified. Windows Git execution currently returns unsupported;
cross-build success is not native platform qualification.
Successful diff/history reports bind the disclosed view to full commit IDs; signed
commit headers are parsed and scanned without verifying signatures or authorship.
If the effective output cap cannot fit even the fixed failure report, exit 2
emits no bytes.

`command-guard inspect --root ABS --scope FILE --policy FILE --request FILE --format json|text` reads only the selected, digest-pinned regular files. It scans every selected file with every configured regex before returning any row. `text` is indented JSON so content remains escaped. Exit 0 means a complete selected-file scan with no configured-rule finding, 1 means a rule, denied path, or operation blocked disclosure, and 2 means invalid or incomplete input, read, limit, or output. Reports carry fixed codes, counts, and no paths, digests, literals, rule errors, excerpts, or findings. Row `file` is the zero-based ordinal in the request's `files` list; `line` starts at one. Search literals must be single-line.

The CUE contract and synthetic structural example are in `schema.cue` and `testdata/valid.cue`. Runnable JSON controls and a digest-pinned sample file are in `testdata/`. Use an absolute root and trusted control files. The policy and scope are separate: both required ID sets must match exactly. Every layer must allow the operation; numeric caps take the minimum; denied prefixes union. Prefixes match whole path components.

From the repository root, run the complete synthetic fixture:

```sh
go build -o /tmp/command-guard-demo ./cmd/command-guard
/tmp/command-guard-demo inspect --root "$PWD/cmd/command-guard/testdata" --scope cmd/command-guard/testdata/scope.json --policy cmd/command-guard/testdata/policy.json --request cmd/command-guard/testdata/request-read.json --format json
/tmp/command-guard-demo inspect --root "$PWD/cmd/command-guard/testdata" --scope cmd/command-guard/testdata/scope.json --policy cmd/command-guard/testdata/policy.json --request cmd/command-guard/testdata/request-search.json --format text
/tmp/command-guard-demo help
```

The sample rules detect only the literal synthetic marker `SECRET`. A zero-finding sample report says nothing about real secret or PII coverage.

The positive CUE fixture includes report rows and validates with `cue vet cmd/command-guard/schema.cue cmd/command-guard/testdata/valid.cue`. Local negative checks on 2026-10-23 returned exit 1 for `_path & "CON.txt"` (reserved basename) and for a `#Request` with `kind: "read-range"` plus `literal: ""` (field not allowed). CUE and Go cap rule patterns at 2048 bytes; Go additionally enforces UTF-8 safety, regex compilation, case-folded duplicate paths, and all file I/O rules.

This is a read-only file inspector, not a shell proxy, authorization gate, or proof that secrets or PII are absent. Regex coverage is configured by the caller. The CLI accepts selected-file controls and has no recursive discovery, arbitrary stdin payload or model path. Default mode runs in-process regex checks; the optional pinned scanner below is a subprocess with host ambient authority. `os.Root` resists escapes; component-by-component opens reject observed symlinks and compare opened file identities. Hardlinks, mounts, and concurrent renames of the same inode are not sensitivity boundaries. Windows special-file behavior requires native qualification. A selected regular file can still change while being read; the request digest pins disclosed bytes. Output truncation counts omitted rows, while all selected bytes are scanned first. Portable paths are limited to printable ASCII and denied prefix matching ignores ASCII case on every host. This can block two distinct case-sensitive names; Unicode path aliases remain unsupported. `bytesRead` includes an optional one-byte cap probe, while `bytesScanned` counts only bytes evaluated by configured rules. Input and scan failures withhold rows before output begins; a writer that fails mid-write may have delivered a partial prefix of already validated rows. A write failure returns exit 2 and does not retry or claim atomic delivery.

Optional external scanner: set scope `requiredScannerIds` to `["gitleaks"]` and policy `scanner` to the fixed `gitleaks-stdin-8.30.1` profile with a trusted absolute executable path, its lowercase SHA-256, `timeoutMillis` (1–30000), and `maxReportBytes` (1–1048576). Both controls must be present together. The guard verifies and copies at most 128 MiB of executable bytes into a private directory, runs the pinned version and stdin recipe from that copy, and applies one deadline to preparation and every selected-file scan. The scanner receives exact selected snapshots after all configured regex checks pass. Its report exposes only aggregate counts and fixed status/code values. A finding blocks disclosure (exit 1); an unverifiable scan, including a zero-byte scanner receipt for nonempty input, withholds all rows (exit 2). No report files are requested. Normal cleanup removes the temporary executable/HOME; cleanup failure holds and may leave private artifacts. The profile checks raw text with Gitleaks embedded default rules. It does not qualify archive/encoded content, exhaustive secret absence, arbitrary scanner configuration, subprocess filesystem or network confinement, or a memory ceiling. The operator must trust the source executable and protected controls. Native support is separately qualified per host.


For a protected policy, the additional object has this shape. Replace the two
placeholder strings with the trusted absolute path and independently verified
64-character lowercase executable hash before validation:

```json
{
  "kind": "gitleaks",
  "profile": "gitleaks-stdin-8.30.1",
  "executable": "<protected absolute executable path>",
  "sha256": "<verified executable SHA-256>",
  "timeoutMillis": 5000,
  "maxReportBytes": 1048576
}
```

Assign that object to `policy.scanner` and add `scope.requiredScannerIds: ["gitleaks"]`.
Keep both files outside candidate write authority in a real deployment. Removing the
required scanner to clear a failure changes policy; it is not a scanner repair.
The guard neither downloads nor installs the executable. Read `externalScanner.code`
for fixed failure categories. `attemptedFiles` and `bytesSubmitted` describe proposed
inputs; `invokedFiles` counts processes that started; `completedFiles` and
`bytesObserved` require validated scanner evidence. Version probes are not file calls.

The [native qualification receipt](../../docs/evidence/command-tools/2026-10-24/README.md)
covers the tested Mac profile. Broader scanner/PII/format coverage and other native
platforms remain in the delivery work order. Gitleaks' default allowlists are part of
this exact profile, and inline `gitleaks:allow` comments are ignored on the controlled path.

Before relying on a scanner setup, check a public input and an independently known
positive canary. The bundled Go fixture emits deterministic fictitious test material:

```sh
go run ./cmd/command-guard/testdata/scanner-canary.go > /private/your-trial/canary.txt
```

Use your own private existing trial directory, select that file in a new request with
its actual SHA-256, and run `inspect` with synthetic copies of the example controls
plus the pinned scanner. Expect exit 1, `scanner_finding`, at least one external
finding and zero rows. A configured regex may block first; that proves the regex path,
not the external scanner. Keep production controls intact when qualifying separately.
Generic-looking passwords can miss the embedded defaults. A passing canary proves
that one known case was detected on that path, not broad sensitivity or secret absence.
