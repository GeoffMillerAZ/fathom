# Exact scanner observations within one invocation

GT13b / OR, 2026-10-24. Extends [GT-C4](governed-command-tools.md#gt-c4-economical-and-faithful-output)
under [0039](../intent/decisions/0039-governed-command-tools-preserve-evidence-and-authority.md).
Receiver: the existing synthetic file/Git inspection workflow, including byte-equal
snapshots at different selected paths. This is optional command-local work in W1,
within its five-day/zero-paid-inference budget. No new implementation slot.

I1/I5/I11/I13/I18 apply: stdlib-only, CUE/Go agreement, hermetic race tests,
downward imports and consumer-owned processes. No new foundation or exported API.
0039's loss-of-evidence/authority triggers have not fired from a production exploit;
review found a missing internal process-start witness, addressed before reuse.
Existing CLI presentation and AK-CLI-02 divergence remain. This adds no UI renderer.

## Selected design

**OR-1: opt in, preserve the default.** Scanner policy gains optional
`reuseEqualInputs: bool`, default false. Disabled mode emits the existing report
shape and performs every existing scan. Independent required scanner/regex scope,
binary snapshot/hash/version, fixed argv/environment and limits remain mandatory.
Every invocation establishes these again, even when its inputs duplicate an earlier
invocation. No disk cache, network service, global state or cached permission.

**OR-2: require actual start.** A valid version probe and completed input scan each
need `started=true` as well as existing exit/log/JSON/deadline checks. A plausible
clean result without this witness holds as `execution_incomplete`. The production
runner already reports Start failure; this repairs the private runner contract and
receipt law, not a demonstrated repository-controlled bypass.

**OR-3: exact, owned observations.** Index complete zero-finding observations by
SHA-256, then compare exact bytes before reuse. Retain private copies, never aliases
that a later runner call can mutate. Bind to this invocation's prepared executable,
version, recipe, environment and captured input only. Finding/invalid/partial/timeout
results are never cached. Capture initial digests before runner calls; check each
current input before use and after its scan, including retention-ineligible inputs.
Check the full corpus once more before successful return. This detects persistent
mutation before disclosure with linear hashing work, rather than rehashing the full
corpus at each step. Hold rather than attach results to changed snapshots. A malicious in-process
runner can fabricate evidence; it is a trusted test/transport seam, not a sandbox.

**OR-4: bound the optimization.** Retain at most 256 entries and 8 MiB of copied
content. When either bound prevents retention, perform the normal required scan.
Never skip input checks or call uncached inputs unsupported because a cache is full.
Existing corpus/report/process bounds remain. No secure-memory-erasure claim.

**OR-5: distinguish logical coverage from processes.** Actual attempted, invoked,
completed, submitted-byte and observed-byte counters retain their current meanings.
When enabled, add `externalScanner.reuse` with closed shape
`{profile: "invocation-exact/1", files: int, bytes: int}`. Its files count reused
scanner-input units, including the Git profile's captured payloads/representation.
Successful coverage requires `completedFiles + reuse.files == expectedFiles`.
When reuse is absent, successful coverage requires `completedFiles == expectedFiles`.
Reuse counts never increase physical process counters. No public input digest or
cached source body. Version probes remain outside input-process counters.

**OR-6: preserve every hold.** Check the aggregate deadline before and after lookup.
Mandatory regex checks still cover all logical inputs. A later finding, malformed
report, changed snapshot, failed process, deadline or cleanup failure withholds every
source row/selector as before. Compact output cannot drop the reuse receipt to fit.
All json/text/compact and file/Git paths preserve its meaning and output caps.

## Acceptance and stopping rules

| ID | Required evidence |
|---|---|
| OR-A1 | Reproduce unstarted version/input acceptance before the repair; both hold afterward with zero false completed calls. |
| OR-A2 | A/B/A clean inputs reuse exactly one observation, preserving all-source coverage and exact actual/reused byte counts. |
| OR-A3 | Forced index collision, same-length input mutation and mutation of an earlier input cannot reuse a result for different bytes. |
| OR-A4 | Findings and errors are not cached; late finding/error after a hit still withholds all file/Git source fields. |
| OR-A5 | Separate invocations rescan; disabled mode preserves wire shape and actual-call counts; zero-length input is covered. |
| OR-A6 | Byte/entry-cap exhaustion scans normally with bounded retained state. Deadline on a hit and cleanup failure hold. |
| OR-A7 | Real enabled file/Git json/text/compact reports satisfy CUE 0.16.1 and 0.17.1; malformed reuse shapes/counts are rejected. |
| OR-A8 | Tiny output limits and broken writers retain existing failure semantics; no source/hash leaks in held reports. |
| OR-A9 | Pinned native Gitleaks A/B/A baseline/reuse trial agrees on semantic results and reduces started scans from three to two. Repeated equal inputs must agree. |
| OR-A10 | Five paired synthetic duplicate/unique trials report calls, actual/reused bytes and elapsed time separately. Unique-input median overhead above 25% requires tuning or parking; no general savings claim. |
| OR-A11 | Independent review, full `task validate`, strict claimed plan proof, source/binary pins, guide-only trial and normal publication gates pass. Other native hosts remain separately qualified. |

Any false clear, byte mismatch on reuse, silently omitted check, counter inflation,
cross-invocation reuse or native equal-input disagreement kills this profile.
Unqualified measurements remain unknown. GT13's broader observation cache and
GT17's accepted-task economics remain open even after this unit passes.

OR-A10's frozen trial uses eight 1,024-byte ASCII files: input `i` begins with
`public-%02d\n` (i=0..7) and is padded with `x` to exactly 1,024 bytes. The duplicate
corpus uses input 0 eight times; the unique corpus uses all eight. Fixed guard
controls allow both lines, 4,096 bytes/file, 65,536 total/output bytes and 32 rows.
Run serially on the selected macOS arm64 qualification host with no concurrent build.
After one untimed pair per corpus, run five pairs of whole guard invocations,
alternating disabled-first/enabled-first order. Retain all monotonic elapsed times,
source/binary/control hashes and results. Unique overhead is the median of the five
`enabled_elapsed / disabled_elapsed` ratios minus one, with a 25% ceiling. Report
duplicate ratios and saved processes separately. A failed semantic/coverage result
invalidates its timing; do not silently drop or replace the sample.

## Source basis and alternatives

At pinned upstream commit `83d9cd684c87d95d656c1458ef04895a7f1cbd8e`,
[stdin](https://github.com/gitleaks/gitleaks/blob/83d9cd684c87d95d656c1458ef04895a7f1cbd8e/cmd/stdin.go)
creates a detector for stdin;
[configuration](https://github.com/gitleaks/gitleaks/blob/83d9cd684c87d95d656c1458ef04895a7f1cbd8e/cmd/root.go)
selects explicit/environment/cwd sources before embedded defaults;
[fragmentation](https://github.com/gitleaks/gitleaks/blob/83d9cd684c87d95d656c1458ef04895a7f1cbd8e/sources/file.go)
and [detection](https://github.com/gitleaks/gitleaks/blob/83d9cd684c87d95d656c1458ef04895a7f1cbd8e/detect/detect.go)
inform the existing byte/log witness. Our fixed private-environment profile selects
the embedded rules. Source inspection supports this bounded hypothesis; native
agreement trials are required and do not prove universal scanner determinism.

Reject persistent allow caches, output-only comparison, trusting a digest without
byte comparison, mutable cache aliases and pretending cache hits started processes.
Private bounded copies cost memory; they preserve ownership without duplicating the
full 64 MiB corpus. This leaves broader cache/provider choices open without building
them now. No decision model is appropriate for exact equality, process witnesses,
coverage, limits or permission; all are deterministic checks.
