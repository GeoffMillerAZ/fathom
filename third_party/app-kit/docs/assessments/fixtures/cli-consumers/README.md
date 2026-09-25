# Bounded CLI consumer probe

This is local macOS test tooling, not a new CLI API or permission to execute a
consumer. Obtain the applicable execution authority first. The current consumer
approval excluded real-host checks; a later request for sandboxed help/version/
invalid-input checks remains separate from the synthetic tests below.

## Retained execution

Prepare two explicitly reviewed binaries and a canonical absolute, empty mode0700
output directory outside the sandbox's denied source/user trees. Then, if authorized:

```sh
node docs/assessments/fixtures/cli-consumers/run.mjs ABSOLUTE_BOSUN ABSOLUTE_SURVEYOR EMPTY_PRIVATE_OUTPUT
```

The runner bounds regular-file reads, refuses symlink inputs and snapshots exact
probe/binary bytes with exclusive writes. It executes only those copies and verifies
their hashes before and after. Inputs, command, bounded stdout/stderr hashes, probe
receipt identity and runner outcome remain private. Mutable originals are not executed.
The bootstrap's separately retained `observedRunnerSource` is honestly a source read,
not independent proof of the already-running bootstrap's loaded bytes.

The inner probe allows only help, version and two deliberately invalid inputs.
Before either synthetic or authorized real binary runs, read/write/network canaries
must prove the local sandbox denials. No operational leaf command, user HOME override,
dependency install, installed-binary replacement or consumer source write is involved.
The fixed version expectation remains `v0.1.0-dev`; explicitly review future changes.

Each inner invocation keeps its five-second/64KiB-per-stream bounds. The owned outer
process group has a70-second deadline and64KiB-per-stream output cap; interruption,
timeout and overflow stop only that group. A failed/incomplete receipt cannot pass:
schema, canaries, complete unique case IDs, run arguments and binary identities are
checked. Failed receipts remain failures, never skipped or deleted.

## Reporting is not exit classification

Usage cases require exit2, empty stdout, nonempty stderr and clean bounded/no-ANSI
completion. Additive `dimensions.reporting` and `dimensions.exit_policy` reveal
whether the message and exit policy separately pass. A restored diagnostic at exit1
still fails the original combined acceptance oracle and leaves process status1.
Neither the historical four consumer failures nor their receipts are relabeled.

## Synthetic tests, safe without consumer execution

```sh
node --test docs/assessments/fixtures/cli-consumers/probe_test.mjs docs/assessments/fixtures/cli-consumers/run_test.mjs
```

Tests generate only tiny synthetic shell CLIs in new private temporary directories.
They exercise retained-copy execution after original mutation, tamper/preflight refusal,
complete-report checks, intentional exit1 failures, deadline and output caps. Output
directories are retained and printed. Sandbox execution tests run only on macOS;
other platforms skip them and earn no runtime proof. Pure oracle checks import the
probe without launching subprocesses. No Bosun or Surveyor binary is used by tests.

Source/module provenance, isolated consumer builds, installed-host delivery, real leaf
commands, terminal interaction and usage-classifier adoption remain separate evidence.
