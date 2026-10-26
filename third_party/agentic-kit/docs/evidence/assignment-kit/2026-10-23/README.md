# Assignment Kit delivery

Final implementation candidate: `c5e21ecb8529bcdc16338d590f3909ff0f823a5d`, 2026-10-23.
Initial delivery `0c38e199` was followed by canonical CUE assessment reuse; the Go
API and evaluator behavior of Assignment Kit did not change. A command-local native
model-reference correction followed at `c5e21ec`. The final candidate retains independent
review and adds canonical-weight/artifact/required-profile rejection tests.
Decision [0036](../../../intent/decisions/0036-assignments-compose-goals-evidence-and-stopping-rules.md),
[AS-1..8 specification](../../../designs/assignment-kit.md),
[adoption guide](../../../assignment-kit.md). New library remains experimental.

## Executable acceptance

- AS-A01..08: CUE/Go bounds and fixtures, immutable intent/source and scope pins,
  graph/required/stretch semantics, original evidence traces, calibrated-confidence
  overlay, concerns, exact budget/deadline/rule precedence, deterministic detached
  outputs, input preflight and concurrent use pass their positive/negative tests.
- AS-A09: two catalog-backed domain workflows produce eight synthetic snapshots:
  complete, missing-required continue, stretch-failed complete, open-concern blocked,
  resolved complete, pivot-required, needs-review and stopped. Definition changes
  and independently omitted privacy/counterevidence goals reject.
- Independent [review](review.md) and [guide trial](skill-trial.md) retain original
  findings and fixed dispositions. The last scope allocation/Unicode-whitespace/time
  shape edges were reproduced and independently rechecked.
- Full `task validate` and fresh focused race tests pass, including the existing
  nested modules. Skill metadata validators pass.
- Clean committed-source command and example binaries at `c5e21ec` both report
  `vcs.modified=false`; actual direct audit and all eight snapshots match the trials.

AS-A10 passed: the declared `task downstream-check` completed with exit 0 against
`c5e21ec`, using supported correctness-test capacity 4. AS-1..8 are complete at their
stated library/reference scope. Strict proof after the final AS8 claim passed:
24 total rows, 15 claimed, 15 verified and zero failing; nine CG rows remain deferred.

## Consumer compatibility

The declared `task downstream-check` passed against the exact committed
candidate in disposable checkouts. Original source workspaces retain dirty work.
Prepared dependency manifests reuse the previously inspected uniseg materialization
and applicable module graph; consumer-b's ignored finance fixtures are copied
only into private local scratch. No dependency pin or private fixture is committed
to a consumer. The results below are executions, not inferred passes from configuration.

| Consumer source snapshot | Gate result |
|---|---|
| Fathom `470726f6d4db328ddb0e4e4cad944bc4f0a6272f` | Pass: exact mirror/vendor, CLI/all-package builds, owned race suite including 2,495 command tests, and the unchanged performance guard. |
| consumer-a `a007075fab6ac156be44de46df941cce4a0915ed` | Pass: declared build and full test target. |
| consumer-b `8b8620936afd2ebcff6810baec1ef37528699c1a` | Pass: declared build and full test target. |
| consumer-c `04be1238bae18e9542b135060233412b4abd72ca` | Pass: exact candidate re-pin/re-vendor, build and vendor/lint guards; scratch only. |

The app-kit dependency snapshot is `6883a60d7863eaff38fafcc9209cf5fcf4cadc51`.
Fathom's isolated own app-kit reverse-check is unconfigured and explicitly skips;
it is not a fifth passing consumer. Local diagnostic logs are retained in the private
recovery directory at
`/Volumes/m2ext/backups/agentic-kit/contributions/2026-10-23-command-assignment/downstream/`.

The first aggregate against `0c38e199` passed Fathom and stopped in consumer-a's
architecture tests: their nested `go list` used stale vendor metadata after dependency
manifest preparation. Normal `go mod vendor` in the disposable consumer-a snapshot
resolved it. Consumer-a, consumer-b and consumer-c then passed their
declared targets separately. The later complete aggregate passed at `c5e21ec`;
the initial failure remains recorded and no vendor check was bypassed.

The `0dc3ee5` rerun exhausted Go's unchanged ten-minute package deadline in Fathom
`internal/fusion`, with `TestEnrichTypedEdges_NoGoWorkNoFallback` running for five
seconds when the whole-package alarm fired. All four command-test shards passed.
The earlier same suite at capacity 2 passed in 555.347 seconds. The successful final declared
aggregate used the script's supported `FATHOM_TEST_GOMAXPROCS=4` correctness-test
profile; assertions, race detection, package timeout and separate performance gate
remain unchanged. This is a capacity-qualified run, not a default-profile pass.
The final fusion suite completed in 433.170 seconds and the separate unchanged
performance guard in 37.384 seconds.

## Retained-artifact integration trial

At `2026-10-23T19:11:52Z`, the exact `c5e21ec` library evaluated the real independent
46-tip inventory and retained disposition records. Full record coverage returned
`complete`; omitting one required observation returned `continue`; shrinking the
assignment to 13 goals against the unchanged independent roster was rejected.
Action authority remained false. The criterion checked record presence and shape,
not the truth of semantic review. This replay followed the corrected inventory;
it does not claim the kit discovered the initial roots-versus-tips error.

The private recovery directory contains the runner and inputs plus the output.
Runner SHA-256: `bb4134c5d927cac1bd465fef4ae53e1277be0e4db34ee75101530292a74f2e8c`.
Disposition input SHA-256: `f082986c300bed6dfb84fc5543044cee998e581acf3ea90a92a8031fb99d863b`.
The output's assignment digest is `04627af2c3321e889f4958f21bf6aeec6cf285fe6d4e401f17685c08b76b82d5`.

## Completion and non-guarantees

This kit evaluates scoped caller-declared acceptance. It authenticates no evidence,
validates no source truth or statistical calibration, reserves no shared budget,
executes no stop/pivot/approval, and grants no action authority. An observation
collector, policy authority and actual dispatcher remain application responsibilities.
No provider/Fathom dependency, storage service, installed harness or UI is required.
No real consumer adoption, measured productivity/savings or whole G9/G11/G14
completion follows from these synthetic references and compatibility checks.

## Pre-push allocation-test correction

The first push of the delivery receipt was blocked by the normal full gate:
`TestScopePreflightBoundedAllocations` measured 34 versus 24 allocations with a
two-allocation tolerance. Repeated focused race runs reproduced the failure.
The evaluator rejects both oversized rosters before uniqueness work, but it first
fingerprints the same assignment. Go 1.26.3's `sync.Pool.Put` deliberately drops
entries with one-in-four probability under race instrumentation; JSON fingerprinting
uses that pool. Three measured calls were insufficient to compare that churn.

The test now averages 1,000 calls and retains the original two-allocation tolerance.
No evaluator, public schema, API, gate or hook changed. An independent scratch trial
with 100 calls passed 100 race runs; moving uniqueness before the cardinality guard
made that trial fail at 5,171 versus 55 allocations. The final 1,000-call version
passed 100 focused race repetitions (20.776 seconds) and the full `task validate`.
An independent final 1,000-call scratch mutation also failed as intended:
5,182 versus 55 allocations (exit 1, 47.37 seconds), with no source-workspace edit.
The original test failed 35 of 100 focused repetitions. This is an allocation regression
check, not proof of zero runtime noise or a production memory benchmark.
