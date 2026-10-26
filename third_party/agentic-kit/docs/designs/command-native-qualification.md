# Native file-command qualification

GT4A / NQ, 2026-10-24. Extends [GT-C7](governed-command-tools.md#gt-c7-platforms-performance-and-acceptance)
within [0039](../intent/decisions/0039-governed-command-tools-preserve-evidence-and-authority.md)'s W1 receiver.
Status: implementation admission. This is a repeatable diagnostic over owned synthetic
fixtures, not an installer, scanner certification or completion of the native GT4 matrix.

## Contract

**NQ-1: fixed independent scope.** `cmd/command-qualify` accepts a profile,
absolute executable paths and exact lowercase SHA-256 pins. Profiles are
`files-core/1` and `files-gitleaks/1`. The latter includes every core case plus the
fixed Gitleaks text-profile cases. There is no caller case list, skip or arbitrary
argv. The compiled roster, ordered case IDs and expected outcomes are independent
of candidate reports. A roster digest binds its canonical versioned descriptor:
fixture byte hashes and names, request/format/limits, control mutations, expected
rows and counters, and scanner receipt expectations. Binary pins separately bind
the implementation. A changed material oracle must change the roster digest.
The descriptor is not a digest of every serialized control-file byte or the whole
program. Host paths vary; source revision and runner binary identity remain required
external evidence alongside this descriptor identity.

**NQ-2: trusted executable snapshots.** Executables remain operator-trusted code
with ordinary host authority. Hashes identify bytes, not provenance. Open a bounded
regular file, verify identity and hash, copy privately, and execute only that copy.
Use an owned private working tree, minimal environment and no PATH lookup. Bound
each output channel at 1 MiB, each child at 15 seconds and the suite at 5 minutes.
Process-start, wait, cancellation and output failures stay distinct from completed
candidate exits. These are subprocess bounds, not filesystem deadlines or process-tree
confinement. The driver initiates no network calls, downloads, provider dispatch,
hook or host configuration mutation. Trusted children retain ordinary host authority.

**NQ-3: behavioral oracles.** Each case fixes its candidate exit/status/code,
required disclosure and expected projection. A valid JSON object or exit code alone
does not pass. Reject duplicate/unknown fields, malformed Unicode and trailing data.
Check exact selected row identities/text and relevant counters. Held cases expose no
rows or synthetic source marker. Compact accepts its documented ordinary fallback
and checks equivalent rows. JSON/text/compact tiny caps must emit zero bytes with exit 2.
Scanner successes bind actual start/completion/byte counts; enabled A/B/A has exactly
two completed scans plus one reused unit. A later finding or unsupported input holds
every source row. No universal secret/PII-absence claim follows from these fixtures.

**NQ-4: honest applicability and result.** Freeze required case applicability before
execution. This first roster has no N/A cases. Symlink creation failure, missing
binary, setup/cleanup failure or any unrun case is unknown. A completed observation
contradicting its oracle is fail. Exit 2 if any required case is unknown or preflight,
cleanup or report output fails; otherwise exit 1 for any fail, 0 only for all pass.
Do not change applicability after seeing a failure. Windows reserved names and alias
cases test control rejection without requiring impossible host filenames.

**NQ-5: portable private receipts.** CUE/Go mirror a closed report with version,
profile/roster digest, actual host OS/arch, executable pins, counts and ordered case
results using fixed codes. Include no user paths, arguments, environment, source text,
subprocess diagnostics or arbitrary build settings. Omit build metadata rather than
publish unbounded or unauthenticated provenance. Public receipts are diagnostics,
not signed attestations. External evidence records bind source revision, runner hash,
observation interval and host details. Report output errors never become pass.

## Frozen scope

The implementation's versioned descriptor gives exact IDs and oracle values. Core
must cover read JSON/text/compact, literal search, late regex finding, wrong input
hash, missing required control, denied prefix, traversal, case-alias request, Windows
reserved-name rejection, invalid UTF-8, file-size limit, symlink rejection and all
three tiny output formats. The compact positive fixture must exercise actual tuple
output, not only fallback. Core also checks the ordinary compact fallback.

The case-alias request tests duplicate-selector rejection, not filesystem lookup
equivalence. Host case-sensitivity/alias behavior remains outside this roster.

Scanner scope adds clean and empty input, distinct A/B/A paths with reuse disabled
and enabled, deterministic fictitious credential canary, a late unsupported PDF,
wrong scanner hash, missing required scanner scope, and ambient configuration/inline
ignore resistance. Only pinned `gitleaks-stdin-8.30.1` is eligible. The canary follows
the existing synthetic generator, never a real credential or external service.

Not qualified: native Git, device/FIFO behavior, hostile descendants, stalled
filesystem cancellation, native authentication, installed hooks, concurrent hostile
directory mutation, other scanner versions/formats, model classifiers, production
interception, task economics or other OS/architecture/filesystem combinations.
These remain GT4/5/9..12/17 and require their own independently scoped witnesses.

## Acceptance

| ID | Evidence required |
|---|---|
| NQ-A1 | Fake transport tests prove an allow-all candidate, empty/malformed/duplicate/extended report, wrong counters/rows, missing process start, timeout and channel overflow cannot pass. |
| NQ-A2 | Fixed roster identity/order, strict arguments/pins, bounded executable snapshots, unknown precedence, output errors and cleanup failure are tested. No skip selector. |
| NQ-A3 | Actual native core and scanner profiles run against pinned built binaries. Retain failures and sanitized receipts, not only summaries. |
| NQ-A4 | Actual runner JSON/text receipts and candidate JSON/text/compact tuple/fallback reports satisfy their CUE definitions; invalid success/count/roster and malformed compact cases fail schema or documented Go semantic validation. |
| NQ-A5 | Windows/Linux cross-build plus native Linux/Windows/macOS core witnesses identify actual host/architecture. Missing or failed hosts remain unknown/fail. |
| NQ-A6 | Independent review and guide-only trial, full `task validate`, strict claimed-plan proof and normal push hooks pass. GT4 remains open outside the measured scope. |

Stop or revise on any false pass, private data in a receipt, missing required case,
unbounded buffer, or changed host control. No optional judgment/model site is admitted:
every question in this roster is computable from fixed fixtures and observed bytes.
Owner: kit maintainer. Same W1 budget/review date as 0039; no new slot or shared engine.

Hosted native witnesses build both tools at the recorded workflow/source revision
and compute their byte pins. This binds the observation to those built bytes; it is
not independent trust validation of the source or oracle. A separate additive native
workflow leaves existing required CI, repository gates and host configuration intact.
