# Retained CLI probe preparation

G5 verification tooling, not new consumer execution or completion of the terminal
target. The [historical Bosun/Surveyor result](2026-09-14-cli-consumer-errors.md)
and its four usage2 failures remain unchanged. Consumer execution approval is pending.

## Change and boundary

The original probe hashed executables then invoked their mutable paths, without
retaining its own executed source. The new [owned runner](fixtures/cli-consumers/README.md)
reads each bounded regular input once, writes exclusive private copies, executes the
retained probe/binaries and rechecks their identities afterward. It records command,
exit, deadline/overflow, bounded output hashes and the inner receipt identity.
Receipt acceptance requires the expected schema, consistent canaries/status and the
complete unique case set with matching binary identities/arguments. Empty or missing
checks cannot become a pass. The bootstrap source snapshot is explicitly observational,
not independent evidence of that already-running process's loaded bytes.

Usage output now separates `reporting` from `exit_policy`. Both must pass for the
unchanged combined usage2 acceptance oracle. An error message at exit1 may prove the
reporting dimension but still fails the invocation and whole probe. No classifier,
public API, dependency pin, consumer file or installed binary changed.

## Synthetic verification

Nine tests pass on macOS, with zero skipped. They use only generated shell fixtures:

- Pure result checks preserve help/version, refusal and usage expectations; timeout,
  truncation, ANSI, stdout contamination, signals and spawn errors cannot pass.
- Unsafe/unbounded inputs and non-private output are refused; retained-byte tampering
  fails before launching. The failure receipt records that no execution started.
- The complete sandbox probe passes13 checks using retained synthetic binaries even
  after their originals are replaced. All three sandbox canaries precede execution.
- A separate synthetic exit1 fixture restores diagnostics but deliberately keeps
  four usage failures and the nonzero runner outcome.
- Deadline and output-cap negatives retain failure evidence; stdout is capped at the
  selected limit with a matching digest and only one overflow diagnostic.
- Eleven altered-report controls reject wrong schema/kind/canaries, missing/duplicate
  checks, omitted runs, wrong binary/arguments and inconsistent outcomes.

The final tests execute retained copies in `/Volumes/m2ext/appkit-cli-retained-proof.FDLo3V/`.
`tests-receipt.json` SHA-256: `c4cbe796dc161af7c3e1df6fe418398dac3cafc01e272f85e1f06550610d30f6`.
It retains exact commands/environment, before/after source hashes and stdout/stderr
identities; all four source files remain byte-identical. Node26.3.0, Darwin arm64.
The passing synthetic trial is `appkit-cli-owned-test-b3jPv9/trial`; intentional usage
failure is `appkit-cli-owned-test-HHigCG/trial`. Earlier drafts are retained separately.

## What this does not prove

No actual Bosun/Surveyor binary, operational leaf command or real user data was used.
Consumer source/build provenance, sibling-free delivery, installed-host behavior,
usage-classifier adoption and full terminal interaction remain open. Sandbox canaries
and owned process-group cleanup are bounded local controls, not general containment
against malicious escaped descendants. The runner grants no new execution authority.
