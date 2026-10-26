# GT5 differential security review

## Executive summary

Baseline: `bae835f`. Reviewed current uncommitted `cmd/command-guard/**` plus GT-C3a on 2026-10-24. High-risk change: first external scanner execution and its disclosure decision. **Open findings: none confirmed.** Two low-severity receipt/claim defects found during review were corrected. Recommendation: accept this bounded code slice on the supplied gate and native qualification receipts. This is not host-isolation or production authorization evidence.

## What changed

`main.go` now requires matching optional scanner controls and calls the scanner after all selected snapshots pass path, digest, text and configured regex checks. New `scanner.go` copies a bounded hashed executable, checks version, runs a fixed stdin recipe, and reconciles both output channels, JSON findings, byte log and exit code before `main.go` can render rows. The changed CUE schema/fixture, guide and GT-C3a describe that profile. No exported library API was added.

## Findings

**F1, resolved, low — attempted scans reported as invoked.** Earlier `scanner.go` incremented `invokedFiles` and `bytesSupplied` before runner start. A pre-start failure could report delivery of bytes never supplied; incomplete status still withheld content. Current `scanner.go:303-310` separates `attemptedFiles`/`bytesSubmitted` from `invokedFiles`/`bytesObserved`. `scanner_test.go:174-188` covers a start failure.

**F2, resolved, low — absolute cleanup claim.** The first `cmd/command-guard/README.md` draft said no scanner files or reports persist, although `scannerBinary` creates a private copy and HOME/TMP and `RemoveAll` can fail. The guide now says no report file is requested, normal cleanup removes temporary artifacts, and cleanup failure may leave private artifacts while holding the result.

No demonstrated false clear or premature content write remains in the reviewed path.

## Tests and blast radius

The scanner path is command-local: `runWithScanner` -> `scanSelectedWithRunner` -> `scannerCommand`. `scanner_test.go` exercises control mismatch, version/hash failures, malformed and duplicate JSON, byte and log mismatches, caps, timeout, process start failure, and late finding suppression through `runWithScanner`. These are synthetic injected-runner/test-executable cases. I inspected the tests but did not run the full gate or the actual Gitleaks binary. No consumer or exported caller was added; the affected runtime path is this CLI invocation.

## Historical context and adversarial checks

Baseline blame places the selected-file validation and all-or-nothing disclosure in `bae835f`; GT5 inserts the scanner between that established check and rendering. I checked candidate-controlled source text and files, mismatched protected controls, malicious/malformed scanner reports, silent skip/fragment diagnostics, output overflow, executable replacement, ambient config, and late findings. Binary SHA-256 and execution-from-copy bind code bytes within the stated trusted-operator boundary. The fixed child environment/cwd removes candidate config and ignore authority. Exit 0 alone never clears: JSON, complete-byte log, deadline and clean exit must agree. A finding or ambiguous outcome withholds every row.

## Recommendations and limits

Retain the final source identity, gate and native Mac scanner trial separately from this static review. Fathom `review_diff` and `obligations` timed out at the 7-second bound, so this report uses explicit diff/source and baseline blame. Native Windows/Linux behavior, child filesystem/network confinement, process-tree guarantees, hard memory bound, hostile filesystem wall time and semantic completeness of Gitleaks defaults are outside GT5's stated qualification. No actual scanner, network, or private source corpus was used in this review.

## Final byte reconciliation (2026-10-24)

After the initial review, the only production-path edit removed an unused `scanSelected` wrapper. The product still calls `scanSelectedWithRunner` from `runWithScanner`; the GT5A plan anchor now names that reachable function. The separate strict candidate result reported 4/4 claimed rows verified, with broader GT5 still in progress. A deterministic stdlib-only fictitious scanner canary was added under `testdata/`; it does not run in normal inspection.

I independently compared all 19 `source-pins.json` hashes to current files: no mismatch. I parsed `native-trial.json`: 14 cases, each observed exit matches expected, and every nonzero-exit report has zero rows. The evidence README links resolve locally; the private receipt directory exists. Current guide text explicitly identifies the optional scanner as a subprocess and limits the canary to one known detection. I inspected these artifacts; the main thread supplied the full `task validate`, fresh race/CUE, native trial, cross-build and strict-plan run receipts. I did not rerun those instruments or execute Gitleaks. No new regression or misleading completion claim found in the final reviewed bytes.
