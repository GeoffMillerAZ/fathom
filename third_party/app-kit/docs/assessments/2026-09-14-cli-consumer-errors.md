# CLI consumer error-boundary assessment — 2026-09-14

## Result

Actual Bosun and Surveyor binaries pass bounded help/version checks. Both silently
return **1**, not the required usage **2**, for unknown flags and commands. Four
failures remain. The kit's opt-in reporting helper does not change these binaries
or solve complete invocation-error classification. G5 is not complete.

| Consumer | Help | Version | Unknown flag | Unknown command |
|---|---|---|---|---|
| Bosun | exit0; 1,690 stdout bytes | exit0; 25 stdout bytes | exit1; no output; FAIL | exit1; no output; FAIL |
| Surveyor | exit0; 1,601 stdout bytes | exit0; 28 stdout bytes | exit1; no output; FAIL | exit1; no output; FAIL |

All eight invocations had empty stderr and no ANSI, timeout or output truncation.
Version output includes `v0.1.0-dev`; this is a development identity, not a release pin.

## Reproduction and safety boundary

[probe.mjs](fixtures/cli-consumers/probe.mjs) accepts two already-built absolute binary
paths and an existing empty owned output directory. It invokes only `--help`,
`--version`, `--appkit-probe-unknown-flag` and `appkit-probe-unknown-command`.
Five-second deadlines and 64 KiB per-stream bounds apply. Children receive only a
fixed system PATH, `NO_COLOR=1` and `TERM=dumb`, in empty owned working directories.
No HOME override, user configuration, actual leaf command or real business data is used.

macOS `sandbox-exec` denies all network/file writes and reads under `/Users`,
`/Volumes/m2ext/code`, `/Library/Keychains` plus a synthetic sentinel. Before consumer
execution, permission-denied read/write and a blocked connection to a parent-owned,
self-checked loopback server all pass. Both consumer working directories remain empty.
These are three demonstrated controls around trusted smoke checks, not universal
malicious-process containment. The harness retains all failed expectations and exits1.

Private receipt: `/tmp/appkit-cli-consumers-baseline.wyqqZN/checks/report.json`.
Host: Darwin25.6.0 arm64; Go1.26.3; Node26.3.0. The report has 13 checks:
three sandbox controls, eight invocations and two empty-directory checks; 9 pass/4 fail.
Raw bounded output remains in the private receipt, not distributed source.

| Artifact | SHA-256 |
|---|---|
| Probe | `c1035abf2ab26ed631e8d98e7a700c1633dd5247eb590296151ff47929ffaee0` |
| Report | `f95d6791bf53f9e3dc323365558c103e2767309b72d5d07010e92b6f22c6d8f6` |
| Bosun binary | `8ce50bdab17f0a260b49899c3cd01762c3993c20a6d2ec0eb4aec4dd28fc5f41` |
| Surveyor binary | `23bc44cb9adc041321fdd5db9bc1946c78001160914cc0d63a48b03cd957c56d` |

Built from the existing clean `kits/env-kit` and `kits/test-kit` subtrees of
platform-engineering HEAD `7ff0104b5aa391b62a0ae1925f65161de222552f`:

```sh
# In each leaf module; binary is bosun or surveyor, with its matching cmd path.
GOWORK=off GOPROXY=off GOTOOLCHAIN=local CGO_ENABLED=0 \
  go build -mod=readonly -trimpath -buildvcs=false \
  -o /tmp/appkit-cli-consumers.EWgund/bosun ./cmd/bosun
```

These are development builds resolving app-kit through the existing local replace,
not isolated release builds. Both consumer module/sum hashes and subtree status were
unchanged after the builds/probe. Source, dependency pins and global configuration
were not edited; Fathom's pending unrelated pin decision was not used as authority.

## Cause, reusable fix and remaining work

At baseline, both process entrypoints passed no-op `fang.WithErrorHandler` callbacks to prevent
duplicate curated diagnostics. Cobra framework failures never reach the curated
leaf adapter, so their only diagnostic is suppressed. Meanwhile, app-kit's existing
`Code` intentionally maps unclassified errors to1. A nonzero-only unit assertion
cannot detect either problem.

[ADR-0039](../adr/0039-explicit-cli-error-reporting.md) adds opt-in `Reported` and
`UnreportedErrorHandler`: acknowledgement is independent of exit classification.
Single wrappers/all-reported joins may suppress; a mixed join must delegate intact.
Existing `Code`, `Exit`, `Execute`, consumer options and pinned dependencies stay
unchanged. Consumers must actually report a diagnostic before marking it.

Pinned Cobra v1.10.2 exposes untyped errors from command discovery and required/group
validation. Pre-hooks also precede required-flag checks. Fang v2.0.1 creates commands
inside Execute and renders errors before returning. Neither error-message matching
nor a `RunE started` flag safely recovers provenance. No local fork/cache patch is made.

The owner subsequently authorized error-reporting-only consumer changes, explicitly
preserving exit codes and forbidding real-host probes, dependencies and commits.
Both working-tree entrypoints now use `UnreportedErrorHandler(nil)`; their leaf
adapters wrap already-rendered diagnostics with `Reported`. Synthetic in-process
tests exercise the actual adapters and entrypoint execution seams: plain errors,
mixed joins, unknown flags/commands and already-reported failures render once.
Bosun retains leaf codes1/2; Surveyor retains its legacy nonzero-to1 mapping;
unknown syntax remains1 in both. Four focused race suites and targeted vet pass;
independent review and repeated focused tests found no actionable issues. The
platform-engineering changelog records the bounded change alongside existing edits.

This is actual source integration, not an installed-binary or host-runtime pass.
The historical9/13 acceptance receipt remains unchanged: its desired usage2 contract
is still unmet. Typed invocation provenance and complete machine/host proof remain
separate work; no string-matching classifier or relaxed usage oracle was introduced.
PTY interaction/restoration evidence remains a separate [terminal receipt](2026-09-13-terminal-reference.md).

### Typed flag provenance seam — 2026-09-14

The new kit-local `flag_provenance_test.go` exercises pinned Cobra/pflag through public
`Execute`, `SetFlagErrorFunc` and `UnreportedErrorHandler`. `NotExistError`,
`ValueRequiredError`, `InvalidValueError` and `InvalidSyntaxError` each retain
`errors.As` identity, reach one renderer once and preserve the existing default exit1.
Negative controls prove unknown-command discovery and deliberate `RunE` failures never
enter the flag hook or acquire flag provenance; an explicit `Exit(2, err)` remains2.
This is a test-only bounded seam, not usage classification. Unknown commands and
required/group validation remain untyped; the historical9/13 usage2 failure still stands.
ADR-0039's provenance trigger is evaluated for this flag-only seam; incomplete invocation
coverage does not justify a default classifier or a consumer exit-code migration.

## Verification of the additive kit change

- `go test -race -count=1 ./appkit/cli/command` and uncached `./appkit/cli/...` pass.
- `GOPROXY=off go test ./...` passes; CLI/MCP `go vet` passes. These root-module
  checks do not cover separately nested native spikes or consumer product workflows.
- Six new top-level tests cover identity/explicit exits, chain/join suppression,
  default/custom handlers, real Execute paths, caller override and NO_COLOR.
  Pinned unknown-syntax exit1 assertions preserve compatibility; they do **not**
  replace the independently failing consumer usage2 acceptance oracle.
- MCP ADR/index/document parity checks pass; intent link/budget and Unreleased
  checks pass (not a full agent-host/wiring doctor). Probe review found no issues.
- The first final-check attempt got workspace EPERM under login-shell initialization;
  rerunning without login initialization passed. No shell/global configuration changed;
  the cause of that invocation failure was not established.

Verified helper SHA-256: `02226a888fa2f02b51afa11502c0f38f2c37c345cb591a0b5a44987a19fb27a4`.
Verified test SHA-256: `eda5a38380e041ef5019c8cd1443a64047305fd0ae0c91b6e315a23693b46a67`.

Follow-up: the [retained execution runner](2026-09-16-cli-retained-probe.md) now prepares
stronger future receipts and separates reporting from exit policy. Its nine tests use
synthetic CLIs only; this historical consumer result and usage2 failures are unchanged.
