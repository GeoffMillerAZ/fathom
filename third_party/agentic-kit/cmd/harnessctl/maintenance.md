# Qualifying an advisory maintenance receiver

Experimental HMT profile. [0043 and HMT](../../docs/designs/harness-maintenance.md)
define the wire, authority, storage and acceptance. This is build/test guidance,
not hook installation or permission to enroll a key or request authentication.

The receiver changes only independently permitted advisory sampling values for
one registered project. Mandatory floors remain intact. Public reports carry
fixed diagnoses and process/write counters, never a reusable allow token.
No installed coding-agent dispatcher consumes these decisions yet.

## Start with synthetic tests

Use a physical absolute scratch directory outside the module. On macOS, resolve
the directory first: `/tmp` and `/var` can be symlink aliases, while controlled
file inspection requires canonical paths. Do not change global environment settings.

```sh
mkdir -p /absolute/protected-scratch/hmt
(
  export TMPDIR=/absolute/protected-scratch/hmt
  export GOTMPDIR=/absolute/protected-scratch/hmt
  go test -tags harness_maintenance -race -count=1 ./cmd/harnessctl
  go test -count=1 ./cmd/harnessctl
  cue vet ./cmd/harnessctl/...
  go build -tags harness_maintenance -o /absolute/protected-scratch/hmt/harnessctl ./cmd/harnessctl
  /absolute/protected-scratch/hmt/harnessctl help
)
```

Tests use disposable policy fixtures, in-memory synthetic signing keys, injected
clocks/entropy, owned-file storage and fake native processes. They cover the
lifecycle, denial/replay, rearm/recovery, clocks, ambiguous writes, wire bounds and
privacy. No real authentication, model call, enrollment or installed hook is used.
The fake signer is not a production runtime option. A pass covers this tested scope.

For an observable positive toy task, run the actual injected CLI lifecycle:

```sh
go test -tags harness_maintenance -count=1 -v ./cmd/harnessctl \
  -run '^TestMaintenanceCLI(FlowAndPrivacy|ConsumeAndDeny|LateApprovalDoesNotExtendLease)$'
```

Expect three named passing tests. They exercise signed synthetic entry/check/end,
denial with consumed nonce, and delayed approval that cannot extend the lease.
Use the same scoped physical TMPDIR as above. They do not invoke the real SDK.
The README's HC8 toy deliberately forbids maintenance. The candidate below cannot
run against it unchanged; changing a candidate cannot create the missing delegation.
No production-shaped fake approver or enrollment shortcut is supplied by this guide.

The process fixture creates a protected home-based preparation root. Every ancestor
must be root/current-user owned and not group/other writable; the selected root is
private/current-user owned. Same-UID candidate writes still require deployment
isolation. File modes cannot prove that isolation. Unsupported hosts cannot qualify
the macOS process/approval profile through cross-compilation.

## What an operator must supply

| Input | Independent authority |
|---|---|
| Controller path and raw SHA-256 | Protected operator/dispatcher selection, not the proposal's own hash |
| HC8 registration and signed sources | Existing [configuration guide](README.md); source-policy Ed25519 keys grant no maintenance authority |
| Helper executable/hash and preparation root | Separately built, inspected and protected macOS binary and namespace |
| Native approver roster | Trusted enrollment of existing user-presence protected Secure Enclave P-256 keys, OS UID, public point, roles, controls and duration cap |
| Project and state roots | Exact canonical registered project, existing protected local POSIX state directory, candidate excluded |
| Candidate JSON | Exact advisory changes and reason; it supplies no authority |

Use `#MaintenanceController` in `maintenance_schema.cue` and HMT-2 for the full
closed controller record. Its native selection includes `profile`, `executable`,
`sha256`, `preparationRoot` and `timeoutMillis`. Do not invent an issuer from a
filename or copy test keys into a real roster. A controller/pin supplied by the
same caller does not authenticate bootstrap.

Virgin initialization pins immutable `BINDING` to controller/registration bytes
and epochs, host/project/principal and state directory. Controller rollover is
not implemented. A different controller with its own matching pin cannot take
over an existing receiver through init or recovery.

The [macOS helper](native/darwin/README.md) is not an enrollment tool. `stamp` is
read-only. `plan-check` validates framed syntax and returns `noAuthority:true`.
`preview` is a visibly labeled synthetic dialog with only Close. None authenticates
an approver. `approve` can request real authentication and is outside a build-only
trial. Missing hardware/key/UI/protection stays unavailable. Windows/Linux real
native approval profiles remain unimplemented.

## Proposed receiver task

This candidate requests 60 seconds. The advisory control must already permit
maintenance in every signed layer, stay inside every rate range and appear in
the approver's control roster. The example cannot waive a mandatory floor.

```json
{"version":"harnessctl.advisory-candidate/1","purpose":"maintenance","approverId":"operator","durationSeconds":60,"reason":"Investigate advisory-hook latency","controls":[{"id":"review","samplePPM":750000}]}
```

After independent prerequisites and host qualification exist:

```text
harnessctl maintenance init --controller ABS --controller-sha256 HASH --project ABS --format json
harnessctl maintenance prepare --controller ABS --controller-sha256 HASH --project ABS --candidate ABS --format json
harnessctl maintenance enter --controller ABS --controller-sha256 HASH --project ABS --format json
harnessctl maintenance check --controller ABS --controller-sha256 HASH --project ABS --format json
harnessctl maintenance end --controller ABS --controller-sha256 HASH --project ABS --format json
```

Enter durably consumes the nonce before one bounded native attempt. Review exact
old/new rates, purpose, project/principal and expiry. Approval delay consumes the
prepared window. Later boundaries revalidate retained signature, roots/configuration
and wall/continuous deadlines. End/expiry restores the freshly validated baseline.

## Failure and recovery

Exit 0 completes the requested receiver operation. Exit 1 means policy/native denial.
Exit 2 means incomplete/unsupported evidence. A check on locked or locked-origin
pending state returns `rearm_required`, exit 1. A completed native process, verified
signature and committed state are different observations. Output/storage failure
can follow an uncertain effect. Inspect before retrying.

Concurrent calls hold as busy. Interrupted approval cannot reuse the nonce. Corrupt
or contradictory slots, pending writes and observed clock/reboot faults expose no
maintenance values. Recover creates fresh locked state, retaining bounded evidence.
An interrupted recovery can finish only its recorded locked replacement. Missing or
corrupt established BINDING requires protected manual disposition. Never edit slots,
remove locks or delete artifacts to obtain a pass. Valid active state rejects recovery;
repeating init cannot reset it.

Rearm requires a new candidate with `purpose:"rearm-baseline"`,
`durationSeconds:0`, `controls:[]` and fresh qualified native approval. Denial stays
locked. No old lease resumes.

Retain source/binary/tool pins and failures. Separate synthetic receiver evidence,
SDK compile/clock/protocol evidence, attended UI evidence, protected-key evidence,
and installed-dispatcher interception. Build-only development covers the first two;
keep GT9..11 open until their actual host qualification exists.

Current [scoped observations](../../docs/evidence/command-tools/2026-10-24-maintenance/README.md)
retain actual clock/receiver and SDK syntax/cancellation results, failures and limits.
