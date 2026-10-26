# Invocation-local scanner observation qualification

Qualified implementation, 2026-10-24. [OR](../../../designs/command-observation-reuse.md),
0039, GT13B and requalified GT5A define acceptance. The specification was committed
as `7eafe27911522abd91827aec6c97cc4309ba645f` before implementation.
Normal publication gates remain a separate delivery observation.

## Counterexample and selected repair

`TestScannerUnstartedSuccessRejected/version` and `/input` failed before the repair:
plausible clean runner responses with `started=false` returned complete/verified.
The input case counted attempted=1, invoked=0, completed=1. INC-24 tracks the missing
helper witness. Production `scannerCommand` already emits a Start-failure code;
this is not evidence of a repository-controlled exploit or source disclosure.

The repair requires start witnesses before completion. Optional `reuseEqualInputs`
then reuses exact zero-finding observations only within this invocation's prepared
scanner/profile/environment. Actual subprocess counters remain actual. Separate
reuse counts establish logical coverage without pretending a process started.
Private copies are capped at 8 MiB and 256 entries. Full cache means ordinary scan.
Any later failure still withholds all selected source. No persistent authorization.

## Instruments and scope

- Hermetic race cases pass for witnesses, exact counts, ownership/mutation/collision,
  isolation, capacity fallback, deadlines, late findings/failures and output bounds.
- Actual file/Git json/text/compact output and CUE 0.16.1/0.17.1 parity, including
  malformed/count-negative cases, pass with CUE selected through PATH. Absence is an
  explicit unqualified skip, never a pass. CI's required CUE install stays unchanged.
- Native owned synthetic inputs use the previously qualified Gitleaks 8.30.1 binary,
  SHA-256 `ba52fb1bfabbcde42f032afad3d6e0b19dff8ed105229a16e7caa338bbc0e84f`.
  Its [provenance and limits](../2026-10-24/scanner-source.json) remain unchanged.
- A clean pre-implementation guard binary at `7eafe27`, Go 1.26.3 darwin/arm64,
  reports `vcs.modified=false`. It supplies independent disabled-wire comparison.
- [Native trial](native-trial.json): 12 owned synthetic cases, 37 actual reports
  checked under each CUE version, exact disabled-wire parity against the saved binary,
  empty input, separate invocations, late findings/unsupported input and hash/cap holds.
- OR-A10's five alternating pairs passed on the selected Mac with zero build
  processes at preflight. Duplicate inputs used one actual scan versus eight;
  median enabled/disabled whole-command ratio was 0.2861 (about 71.4% less elapsed
  time). Unique ratio was 0.9777, within the 25% overhead bar. Its small negative
  overhead is sample variation, not a general unique-input speedup claim.
- Additional predeclared [legacy comparison](legacy-perf.json): three rotating triples
  on eight distinct 1 MiB inputs (8 MiB total), each fully scanned with first-line
  views. Default and enabled medians added about 2.12% and 2.13% elapsed time versus
  the saved pre-implementation binary, within the 25% engineering ceiling. Default
  wire bytes remained identical. These figures include added integrity work against
  the pre-implementation baseline and apply to this selected synthetic Mac corpus.
- Independent [review](review-initial.md) and [recheck](review-recheck.md) caught and
  closed missing-reuse undercoverage and quadratic mutation hashing. The 32 x 2 MiB
  oracle now observes 128 witness hashes in each mode, linear in corpus bytes.
- The first native job **failed** because the complete-coverage CUE union was
  ambiguous for valid disabled receipts. Its runtime assertions reached schema
  validation, but timings were not retained and are unqualified. The [diagnosis](cue-diagnosis.md)
  and [recheck](cue-recheck.md) replace that union with presence-aware constraints.
  Actual disabled complete/finding file/Git regressions failed before and pass after.
  Original source, failed study and logs remain privately preserved.
- Independent [guide use](guide-trial.md) retained failed repeated-path/missing-scope
  controls, then passed corrected enabled/disabled/text/compact/invocation/blocker
  cases. The guide now explicitly requires distinct paths with equal bytes.
- Full `task validate` with CUE 0.17.1 passed. Fresh isolated-source ingest reported
  `outcome: ok`, zero read failures and 18,289 symbols; strict candidate and claimed
  plan checks prove all eight claimed command rows. Static reachability and native
  passing behavior remain separate instruments. The fleet store was not modified.

[Source pins](source-pins.json) bind the exact five changed Go/CUE files and actual
native binary. Its compiled metadata reports `vcs.modified=true` at baseline `7eafe27`;
the listed source hashes were unchanged before/after the trial. This is a working
candidate qualification, not a claim of a clean checkout or same binary hash after
a later Git commit changes build metadata. Per-invocation policy/scope/request and
wire hashes plus every paired timing are retained in the native receipt.

No five-root library/API changed; downstream checks were not required for this
command-local unit. The build/guide/source/review/full-gate/private-proof records and
original worker revisions are retained under the private dated command-reuse backup.

Windows/amd64 and Linux/amd64 command builds passed. Compilation cannot qualify
Windows/Linux runtime behavior, production
economics, installation, hook interception, broader PII coverage or the full GT goal.
GT13's broader cache and GT17 remain open. No model or decision service participates
in exact equality, process witnesses or permission. Existing AK-CLI-02 divergence
remains; no reusable app-kit renderer change is selected by this command-local unit.
