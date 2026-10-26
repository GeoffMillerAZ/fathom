# Subject preparation: current reading

The current preparation screen retains **Go 1, Python 6, TypeScript 3** candidates.
All are drafts awaiting controller vetting. No BX-44 subject is admitted, C4
remains unrun, C5 has zero fail-before/pass-after replays and awaits Q2 for its
post-cutoff half, and live C7 fleet exclusion is unknown. No plan box changed.
Class-transformer/class-validator are BX-M0-only candidates because C5 fails.
Rich and marshmallow are forced to dev; all other splits remain unassigned.

| Language | Retained preparation candidates | Refused in the measured screen | Retained large band |
|---|---|---|---|
| Go | Testify | chi, cobra, urfave/cli: C6; goose: C3/C6; go-git: C3 in this isolation setup | none; gap remains |
| Python | Starlette, marshmallow, Click, attrs, Rich, pytest | none after corrected native temporary storage | pytest (117,244 lines) |
| TypeScript | redux-toolkit; class-transformer/class-validator for BX-M0 only | Immer: C3/C6; tsyringe: C2/C6; Redux/Remeda: C6; trpc: C3 in this isolation setup | redux-toolkit (86,935 lines) |

Read in this order:

- [TypeScript retries and three new candidates](typescript-r2.md), with [full receipts](typescript-r2.json).
- [Python frozen-lock retries and round-1 deltas](python-r2.md), with [installed distributions and receipts](python-r2.json).
- [D24 C6 recount](c6-r2.md), with [all original and new TypeScript witnesses](c6-r2.json).
- [Click snapshot acquisition](click-snapshot-r2.md), with [the final pager recipe and retest](click-snapshot-r2.json).
- [Go/Python large-band search and gaps](size-bands-r2.md), with [pins and C6/C3 evidence](size-bands-r2.json).
- [Round-1 candidate reading with Part A corrections](candidates.md), [historical receipts moved from manifests](receipts-r1.json), [word-bounded C5 counts](c5-corrections.json), and [the required thirty-log archive](raw-logs.md).

The new pytest and redux-toolkit candidates have no admission manifests yet:
D23 star/age metadata must be observed at vetting. No popularity observation or
held-out split was guessed. Existing historical manifests for chi and urfave/cli
carry explicit C6 refusal notes. No subject source or patch is committed.

## Proof and limits

`go test -race ./schema/bench`, `task cue-vet`, `task fmt-check`, `verify.py --archive`, the final offline-log hash/clock consistency check, and
`command gofmt -l` pass. The gate vets every bench.d manifest.
`task validate` is blocked at the sandbox-denied shared gate lease before its
checks; the full gate is unverified and was not bypassed.

All required frozen Python and accepted TypeScript trials have three fresh
processes with networking denied. Go's original retained Testify times remain
round-1 historical evidence from mutable module mode; readonly Go timing retests
were not requested or claimed. The Go helper and future Go suite recipe use
readonly modules. Native macOS and the exact twelve-core condition remain
unmeasured. Trpc is refused after a complete failed Node 24.21.0 warm-up (the
non-loopback-host assertion and OpenAPI test); its three timed trials are unrun.
Go-git's partial warm-up reached hardcoded upstream fixture requests; no full
warm-up timing or three timed trials are claimed. Failed shared-filesystem and
wrong-runtime attempts were retained as invalid preparation evidence, never
used to refuse a subject or claim a green run.

Round 2 has **42 timed offline trials, 39 green and three Immer platform
failures**. Final offline processes are serialized by the job-local lock. The UTC-start/monotonic-duration cross-check differs by at most 0.027 seconds; elapsed times retain the monotonic readings. Raw offline
logs (including denial probes and refusal warm-ups) are returned as
`bench-subjects-r2-offline-logs.tar.gz`, SHA-256 `2fce30cec9ed7d581df7865312e8d345b322cf3f94add4d0185afc64eb9e1cc7`,
with 73 hash-verified members. This is separate from the required
thirty-log round-1 archive. Tar metadata contains no account names.

Review closure: F1 native locks/scripts and reruns; F2 D24 recount; F3 frozen
Python and export hashes; F4 prior-read dev constraints; F5 C5 exclusions; F6
readonly Go; F7 instruments/log bundle; F8 integrated schema text; F9 snapshot
pager; F10 large-band candidates and Go gap; F11 word boundaries and recount;
F12 standardized timestamps. All are addressed within the stated measurement
limits. D23–D25 are recorded in the schema and bench-prep design.

Verify the returned round-2 bundle with `python3 calibration/bench/tools/verify_r2.py --archive "$archive"`; its structured checksum manifest is [offline-logs-r2.json](offline-logs-r2.json).
