# 0043: Maintenance grants bind native approval to receiver state

Status: accepted
Type: security
Date: 2026-10-24

## Context

GT8 authenticates selected configuration, but cannot arm hooks or authorize a
maintenance window. GT9/10 is the next missing critical path in the owner's full
governed-command request. Hosted CI is unavailable before job startup; this does not
prevent building the next receiver. 0039's W1 budget and two-slot ceiling still apply.

## Decision

Admit [HMT](../../designs/harness-maintenance.md) inside command-local `harnessctl`:
an explicit `harness_maintenance` build, independently protected controller/approver
registration, receiver-issued nonce, exact advisory delta, native signing port and
durable local receiver transitions. No foundation, dependency, daemon or third slot.
The receiver is the kit's owned synthetic harness, not installed coding-agent hooks.

Source-policy keys and native approver keys are different authorities. The receiver
invokes a pinned native helper and verifies its signature over the exact prepared
plan. A caller-provided Boolean, detached approval file or model cannot authorize it.
All required controls remain at their signed floors; only named, permitted advisory
sample rates change. No arbitrary hook-code or configuration promotion is included.

Bind every grant to controller/configuration identities, project/principal/host,
receiver instance, authorization epoch, nonce, exact candidate and rollback snapshot,
approver/role and finite UTC/continuous-clock deadline. Consume the nonce durably
before the native interaction. Crash ambiguity, revocation, changed input, expiry,
reboot and observed clock rollback cannot prolong an advisory exception.

Build a macOS SDK helper using protected P-256 key operations and an exact native
review surface. Enrollment, real authentication prompts, key creation, installation
and global mutation are not authorized implementation tests. Compile/read-only clock
and clearly labeled synthetic preview evidence remain distinct from native approval.
Keep Windows Hello and Linux deployment-owned authority profiles separate and
unqualified until their own implementations/native trials exist.

The first persistent-store profile is protected local POSIX storage with cooperative
kernel locking and an intent/double-slot commit. It is not a database, hostile-writer
sandbox or administrator-rollback defense. Unsupported platforms hold before writes.
No installed-hook claim follows from an authenticated receiver state.

## Rejected alternatives

- Persistent scanner caching first: adds retained-content/key/retention obligations
  before the requested maintenance path exists. Wins if: this path is delivered and
  a separately admitted repeated-invocation workload proves the benefit.
- Reuse HC8 signing keys or biometric success as maintenance authority: wrong purpose
  and no exact transaction binding. Wins if: never for this profile.
- Broad disable flag or arbitrary edits during a time window: loses independent
  mandatory floors and exact scope. Wins if: never. Hook-code promotion needs GT7/16.
- Cached allow or timer-only expiry: misses revocation, restart and rollback.
  Wins if: never; each controlled boundary revalidates.
- Universal native algorithm/host emulation: erases native guarantees. Wins if:
  independent native profiles prove an actual common seam without loss.

## Revisit when

A mandatory floor changes, a nonce replays, a crash/time anomaly widens authority,
native identity or storage protection differs from enrollment, the declared latency
bar fails, two consumers need shared extraction, or 2026-11-21/next release arrives.
Keep GT9..11 and the full goal open until their actual host evidence exists.
