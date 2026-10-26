# Scaffold rollback qualification

INC-23/KI-1, 2026-10-24. Repair qualified on native Linux and macOS. The
[native failure](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40091343269)
at `3e23005` passed CUE and failed the existing replacement rollback test: replacement
`main.go` was removed. The test itself remains; it must not be skipped or weakened.
Every other listed package race suite passed, but the overall gate remained red.

The original implementation recorded `f.Stat()` and closed the file before a callback
removed/recreated its name. Cleanup used `os.SameFile` over the old metadata. Unix
device/inode reuse is strongly supported by that source path and Linux-only loss;
the CI log did not capture inode numbers, so it is not a direct inode trace.

## Required repair and acceptance

- Pin each file using `Root.Open` while its exclusive writer remains open and verify
  both identities. Pin workspace using `Root.OpenRoot` and an identity check. Keep
  pins through rollback/commit; preserve the original writer-close/error ordering.
- Retain the path on pin/stat/read/count uncertainty. For file deletion require
  matching live identity, original mode, exact size and bytes of the actual written
  prefix. Reject a short successful write. Preserve detected in-place user changes.
- Bound ownership resources to five file pins, a workspace pin, parent root and an
  active writer. Release every retained handle on success and failure.
- Keep the existing failing regression; add same-content replacement, workspace
  replacement, in-place edits/mode changes, pin failure and short/count failure tests.
  Existing collision/metadata/write/close/setup/cleanup diagnostics remain required.
- Require focused race tests, independent review, full gate and a new green native
  Linux CI run before checking KI-1. Windows cross-builds remain compilation evidence.

`Root` resists traversal; it does not make `Lstat` followed by `Remove` an atomic
conditional deletion. Pins address reuse after acquisition, not every namespace race
or a same-byte rewrite. The destination still needs caller-controlled write authority.
Do not call this a sandbox or a portable transactional filesystem.

Go 1.26 source inspection found that rooted Windows opens share delete access while
ordinary `os.Open` does not. Use rooted APIs for new pins; this is source evidence,
not native Windows qualification. Upstream context: [os.SameFile](https://pkg.go.dev/os#SameFile)
and [traversal-resistant root APIs](https://go.dev/blog/osroot).

Independent source review found no unsafe-deletion blocker. It identified and
accepted a fail-closed compatibility limit: a restrictive umask/ACL may permit
creation/write but deny the new read pin. Generation then fails and reports the
retained path; it never broadens permissions. This limit is part of KI-S1.

Controller Go 1.26.3 focused race tests passed. A second Mac, Go 1.27.3, passed
uncached focused race tests and vet against an unchanged 2,109-file source snapshot
(`ef27d0be1c4011bf8d6404b01a68e6dced9035c9a1e8ba9e746881190b3a62e3`).
These Mac observations did not independently close the Linux regression or qualify native Windows.

The controller's full `task validate` with CUE 0.17.1 passed after integration.
Strict Fathom verification proved all 12 currently claimed command/interoperability
rows; KI-1 remained in progress pending native Linux. Windows/amd64 `kit-new` built
successfully, which establishes compilation only. [Source pins](source-pins.json),
[helper scope](helper-trial.json) and [independent review](review.md) bind these claims.

## Native Linux closure and portable test refinement

[Run 40093730634](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40093730634)
at `c7633a4` passed required CUE 0.17.1, the full race gate, WASI governance build and
WASM example build on Go 1.26.9 Linux/amd64. The original failing scaffold regression
remains present; `cmd/kit-new` passed in 6.178 seconds. [CI receipt](ci-result.json)
records exact source, job/step conclusions and retained log hash. KI-1 and INC-23
close on this runtime observation plus static proof, not on static reachability alone.

Final review also found `Chmod(0600)` can be a no-op on Windows, where only owner-write
controls the read-only flag. The mode-change test now uses `0400` and restores write
permission for temporary cleanup. Production bytes are unchanged. Focused Mac race
tests and Windows/amd64 test compilation passed for this refinement; neither is native
Windows execution. [Final source pins](final-source-pins.json) distinguish it from the
original qualified snapshot. Upstream: [os.Chmod](https://pkg.go.dev/os#Chmod).

## Recoverable workspace cleanup

[Refreshed audit](contribution-audit.json): 767 unreachable objects and 46 actual tips
exactly match the [previously reviewed and preserved inventory](../../command-governance/2026-10-23/contribution-audit.md).
No new object, open PR or stash surfaced. The completed scaffold worker was removed
only after its original two files matched private backup and integrated source hashes.
Both merged repair branches and three completed helper job directories were removed
after receipt/archive verification. No active work, Git objects, global configuration
or installed hooks were removed or changed. The full GT objective remains open.
