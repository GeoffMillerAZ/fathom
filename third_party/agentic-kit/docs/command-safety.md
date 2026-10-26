# Command safety: offline audit and assessment

The optional [compact-view profile](designs/command-compact.md) preserves selected
rows and scan/omission metadata using versioned tuples only when shorter. It adds
neither a disclosure bypass nor an observation cache; decode by returned version.

For actual bounded file inspection, use the experimental
[`command-guard` guide](../cmd/command-guard/README.md). It reads exact selected file
snapshots and scans all selected bytes before returning a range or literal-match view.
Its registered `git-inspect` profile adds fixed status, immutable object-diff and
bounded history reads after protected metadata/configuration qualification. It has
no shell, write, model or installed-hook path. Configured regex coverage is not
proof of secret/PII absence. The [full GT delivery](plans/governed-command-tools.md)
tracks portable tools, scanners and operator hook controls still to be qualified.

For repeatable native observations, use [`command-qualify`](../cmd/command-qualify/README.md)
against independently trusted, hash-pinned binaries. Its fixed `files-core/1` and
`files-gitleaks/1` rosters report required cases, contradictions and unknowns without
publishing source or subprocess diagnostics. A pass covers only that roster on the
reported host OS/architecture; it is neither authorization nor a general platform
or sensitive-data-absence certification. [NQ](designs/command-native-qualification.md)
defines the qualification boundary and remaining native work.

For the protected multi-project configuration, use the experimental
[`harnessctl` guide](../cmd/harnessctl/README.md). It authenticates exact signed
sources, intersects mandatory floors and validates delegated advisory preferences.
Its snapshot report grants no action authority and does not install, arm or disarm hooks.

The explicit maintenance build adds a [protected receiver](../cmd/harnessctl/maintenance.md)
for exact advisory windows and locked recovery. Its SDK build/clock/syntax observations
do not qualify native consent or installed hook control. Mandatory floors remain intact.

The [governed spawn guide](harness-spawn-economics.md) connects current protected
controls to strict required model/effort/tier/fork checks and optional advice
observations. Required checks stay unsampled; native hook adoption remains separate.

The optional [concern-site guide](command-classifiers.md) adds one typed contextual
question after mandatory file checks, with independently pinned public/synthetic
inputs and policy bytes. Unknown, inconsistent or failed judgments hold every row.
Its selected loopback hop is not local-inference proof. Private inputs, actual
model quality and native runtime residency remain unqualified under GT12.

The first [command governance reference](designs/command-governance.md) is an offline
qualification tool. It reads files you name, counts structured command proposals, and
assesses one proposed typed operation against independently declared constraints. It
does not run commands, read a target file named in a request, install hooks, call a
provider, discover private history, or grant permission. Every report says
`noActionAuthority: true`. An `advisory` assessment is a review input, not a safe or
authorized command receipt.

## Try the synthetic inputs

From the repository root, build once so the CLI's 0/1/2 exit status survives the
launcher. `go run` prints `exit status 1` and exits as the Go launcher, so do not use
its shell status to distinguish a blocked assessment from invalid input.

```sh
go build -o /tmp/command-lab ./cmd/command-lab
/tmp/command-lab audit --manifest cmd/command-lab/fixtures/manifest-direct.json --format json
/tmp/command-lab assess --scope cmd/command-lab/fixtures/scope.json --policy cmd/command-lab/fixtures/policy.json --request cmd/command-lab/fixtures/request.json --format json
```

The direct manifest selects only synthetic JSONL in `cmd/command-lab/fixtures/`.
`manifest.json` intentionally includes a JavaScript wrapper and exits 2 with partial
candidate counts, even though the wrapper contains a command literal.
The sample assessment is `read-range` and remains advisory even if it exits 0.
For a negative assessment, copy the sample request to a private scratch file, change
`kind` to `shell`, and run `assess` with that scratch file. Expect `blocked`,
`unsupported_effect`, and exit 1. Malformed controls, missing sources and incomplete
audit coverage exit 2. The [CLI README](../cmd/command-lab/README.md) describes
the capture format for an optional, already obtained classifier response.

## Audit selected sources

Create a private `command-lab.manifest/1` JSON file. Declare
`expectedSourceIds` independently, then give exactly one `sources` entry per ID.
Each entry has a safe ID, `format` (`codex`, `claude`, or `normalized`) and explicit
`path`. Paths are selectors, not a search request. Select only files you have the
right to inspect. Raw transcripts can contain secrets and private user data: keep
them outside the repo, minimize retention, and do not paste them into issues,
provider prompts, or review artifacts. The tool emits closed families and numeric
source ordinals, not the raw commands or paths. Treat its output as sensitive
metadata if source selection itself is sensitive.

```sh
/tmp/command-lab audit --manifest /private/path/manifest.json --format json
```

Audit counts *proposals* in structured tool calls. Literal `functions.exec` command
candidates are separate frequency hints. Neither count proves a process executed.
All JavaScript wrappers remain incomplete, including those containing literals:
candidate extraction cannot establish the absence of aliased or dynamic calls.
Other unsupported records also make coverage incomplete. Check
`expectedSources`, `loadedSources`, per-source status, byte/record totals and
`failures` before interpreting family counts. Exit 0 means complete parsing of the
declared source set under this adapter, not complete command history or safety.
Sources are capped at 32, 4 MiB each, 64 MiB total, 1 MiB per line and 50,000
records. Split an oversized authorized dataset into separate, explicitly scoped
runs and report the boundary; never silently narrow the expected set.

## Assess a proposed operation

Supply three separate versioned JSON controls: `command-lab.scope/1` declares
`requiredLayerIds`; `command-lab.policy/1` supplies exactly those layers;
`command-lab.request/1` names one typed operation and positive byte/result bounds.
The CLI never discovers policy from the cwd or environment. Layer IDs and scope
labels are caller supplied; they do not authenticate organization authority.
Allowed kinds are `read-range`, `search-literal`, `git-status`, `git-diff`,
`run-check`, `write-file`, and `shell`. Constraints intersect and bounds take the
minimum. `run-check`, `write-file`, and `shell` are held in this offline reference
even when listed as allowed. A classifier capture can add an advisory concern;
it cannot remove deterministic blockers.

```sh
/tmp/command-lab assess --scope /private/path/scope.json --policy /private/path/policy.json --request /private/path/request.json --format json
```

Exit 0 is advisory eligibility, 1 is a deterministic blocker, and 2 is invalid or
incomplete input/output. A missing layer, parse error, unsupported format or failed
classifier is not a clean result. No result certifies that a shell command, file
read, write or hook change is safe or permitted. Authorization and controlled-path
execution remain separate [future work](plans/command-governance.md).

## Fix a finding

Use the fixed `codes` and explicit source/coverage receipt to find the smallest
change to the proposed request or trusted policy source. Inspect the relevant
owner's goal, harness rules, hook configuration and actual invocation evidence
before proposing a policy or hook fix. Record the existing requirement, observed
behavior, gap, patch and negative test. A local project file cannot prove an
organization policy floor, and a configured hook cannot prove it fired. Test the
owning controlled path before claiming enforcement. Keep failed hooks and gates
armed while repairing them; never use `--no-verify`, `LEFTHOOK=0`, host-wide
disarm, or a denial loop as a safety release. A policy exception, install or
maintenance transaction needs its own authority and proof.
