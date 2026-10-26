# Quality-gate doctor review (2026-10-23)

Scope: read-only review of the uncommitted `cmd/quality-gate/doctor.go`, `doctor_test.go`, and `main.go` change against main `1aca786`, UP-1/UP-2, and the existing model and Git contracts. No provider call or adoption proof. The findings below describe the reviewed state before repairs.

## Blocking findings

**F1 - Requested and returned model identities are incorrectly equated.** `doctor.go:117-146` requires the requested model environment value to equal the expected returned model, including for optional shadow sites. Runtime `model.go:196-205,267-271` requires both values to be present, sends the requested model, and attests the response against the expected model. `main.go:494-512` binds a clear to the expected returned model. A provider alias such as `requested-alias` resolving to `resolved-model` is valid when the returned identity and calibration match, but doctor reports `required-model-pin-missing-or-mismatched` and exits 2. The offline CLI probe with those two values reproduced that diagnostic. Require nonempty values separately; use the expected identity for calibration. Update the mismatch test in `doctor_test.go:105-109`.

**F2 - Confidence 1 checks only one calibration band.** `doctor.go:139-150` calls `calibrationStatus` with confidence 1. `foundations/decision/confidence.go:69-77` maps this to the Proceed band, while `foundations/decision/calibration.go:115-136` checks only the selected band's labeled cases. A site with `min_confidence=0.6`, eligible Confirm-band clear labels at 0.75, and no Proceed-band labels can clear a 0.75 runtime response while doctor reports `required-clear-calibration-insufficient-n`. A Proceed pass also does not certify a lower confidence response. Capture one evaluation time across sites, inspect each reachable band at or above `min_confidence`, and report whether *any* clear band is eligible. State that an actual response is checked again at its own confidence. No live-model quality claim follows.

**F3 - Local-policy doctor can certify an unusable Git path.** `doctor.go:61-73` labels Git ready after `trustedGitPath` resolves an outside path. With `--policy-file`, `loadPolicy` does not invoke Git. A disposable repo run with a valid local policy and `--git-bin /usr/bin/false` exited 0 and printed `trusted-git pass reason=external-git-resolved`. Enforcement would be unable to run Git plumbing. Check the external executable with a bounded, side-effect-free Git invocation before reporting readiness, including for local policies.

## Other scope and evidence

The command rejects remote/cache/snapshot flags, sanitizes credential values in the inspected reports, and labels local policy untrusted. The text report now names at most 20 quoted unselected paths and points to JSON for the full list, resolving the earlier text-visibility gap. No fresh fail-open enforcement path was found in this bounded review.

Fathom search and source reads identified the runtime contracts. The composed `review_diff` did not return before its wait was stopped; obligations were raw and incomplete (0/10 compiled sections). The three findings have local source or executable evidence, not a clean Fathom review verdict. No full gate was run for this read-only review.

## Follow-up disposition (revised uncommitted doctor)

- **F1 resolved in the reviewed revision.** `doctor.go:128-158` now checks requested and expected model values for presence separately, passes the expected returned identity to calibration, and accepts a distinct requested alias. `TestDoctorAliasAndEligibleConfirmBand` exercises this case.
- **F2 resolved for configuration eligibility.** `doctor.go:40-45` captures one UTC evaluation time. `eligibleClearCalibration` checks up to three distinct confidence bands reachable at or above the site's minimum, returning pass if any clear band qualifies. The report says an actual future response is checked again at its own confidence. The focused test uses qualifying Confirm-band labels and no qualifying Proceed band.
- **F3 remains open after a partial repair.** `doctor.go:74-80` now runs a bounded `--version` probe and rejects `/usr/bin/false`. It ignores the command's output: a valid local policy with `--git-bin /usr/bin/true` still exits 0 and reports `trusted-git pass` and overall doctor pass. The offline CLI probe reproduced this. Require recognizable, nonempty Git version output before reporting readiness; add a `true` regression. This checks usability, not provenance of the host-installed binary.

Verification for this follow-up: `go test -race ./cmd/quality-gate -run 'TestDoctor|TestTextReportUnselected' -count=1 -timeout 90s` passed; `cue vet -c -d '#DoctorReport' cmd/quality-gate/schema.cue cmd/quality-gate/testdata/doctor-pass.json` passed. The `#DoctorReport`/`#DoctorCheck` shape and fixture cover stable typed JSON fields, while the live `evaluated_at` field is intentionally time-dependent. No full `task validate`, provider call, or hook/CI adoption test was run by this reviewer.

## Final F3 disposition (repaired uncommitted doctor)

**F3 resolved in the reviewed revision.** `doctor.go:74-83` now runs bounded, side-effect-free repository plumbing and accepts only `sha1\n` or `sha256\n` from `git rev-parse --show-object-format`. The regression covers both `false` and `true`. An independent offline CLI probe against a valid local bootstrap policy returned exit 2 and `trusted-git-unavailable` for `/usr/bin/true` and `/usr/bin/false`; `/usr/bin/git` returned exit 0 and `external-git-usable`. This proves the local readiness distinction in the probe, not provenance of the host Git binary or enforcement/adoption. The root agent owns the full gate.
