# Assessment profiles and prompt screening: verification

Date: 2026-09-16. Candidate is this change on `docs/intent-excellence-assessment`,
based on `32c5f814ee1dedbc66812415dd3d0ab3a458205f`.
Design: [0012](../../../intent/decisions/0012-composable-engineering-assessments.md),
[engineering assessments](../../../designs/engineering-assessments.md).

## Implemented and checked

- Existing fitness registry: snapshot checks on ingress/read, NaN rejection and an
  injectable clock. The mutation regression failed before repair (INC-15).
- Experimental profiles: required/scored separation, explicit unknown/N/A, admissible
  evidence methods, definition/subject binding, expiry, bounded inputs and deterministic
  fingerprints. Known terminal findings need artifact references; references remain
  caller-declared and unresolved by the library.
- Existing guardrails kit: explicit refusal-policy defaults plus a one-call classifier
  adapter and held continuation. Strict JSON rejects unknown/missing/duplicate fields,
  invalid Unicode, malformed/truncated/tool output; ambiguity/errors never allow work.
  Receipts distinguish unavailable response/usage/attempt evidence from known values.
- Exact shipped refusal defaults are compared against their CUE value. Actual Go JSON
  for allowed, held and failed screens and profile outputs is checked against CUE.
- The full `task validate` gate and focused `-race -count=1` suites passed. No new
  dependencies, stable schema changes, gate changes or paid LLM calls were needed.

## Executed reference, not production quality

`go run ./examples/engineering-assessments` and its tests produced:

| Subject | Required outcome | Soft score | Reason |
|---|---|---|---|
| Retained resume_as_code synthetic benchmark receipt | unknown | null | three costs remain missing |
| Scripted screening plus strict effect gate | pass | 1 | blocked prompt cannot run; benign counter executes; destructive-marked counter is denied independently |
| Same security profile with effect gate disabled | fail | null | the unauthorized counter runs, even though benign utility passes |

`TestNegativeControlCannotHideBehindBenignScore` pins the final row. Handler counters,
denial events and in-band error results are checked; configuration flags and a model's
claim of safety are not substituted for effects. All handlers only increment integers.
`TestDomainCollectors` verifies retained artifacts, reconstructs results after JSON
round-trip and checks the wire against CUE. Repeated runs are byte-identical.

These are deterministic, synthetic wiring checks. They establish no live classifier
accuracy, calibrated probability, actual latency/cost, authenticated evidence, universal
security guarantee, native-host interception or production model recommendation.
Caller-owned classifier eligibility, budget and context-cooperative behavior are explicit
preconditions, not things a constructor or a new attestation boolean could establish.

## Downstream compatibility

Actual consumers were not modified. Isolated candidate copies/module overrides used
offline dependency resolution and readonly module files. New default-vendor production
adoption is not claimed. This reruns the earlier [baseline comparison](../../benchmarks/2026-09-16/README.md).

| Consumer revision | Result |
|---|---|
| throughline `a007075fab6ac156be44de46df941cce4a0915ed` | build and tests pass, including architecture subprocesses |
| fathom `68a534854f1bb8d4e68b2377a2ff4d198e6bef48` | build passes; same `TestNarrowProfileSchemaBytes` failure: 34,821 exceeds 34,000 |
| growing-gardens-bi `7503dd1c270bf6ceb72d42c742f507b9513db0dc` | build passes; same 14 finance failures from missing `../../tmp/financials.json` |
| resume_as_code `7a6c56a26dd10a7cf80128d23ef13bab90377ae8` | `cmd/llm`, `cmd/redteam`, `cmd/ops` pass with `-race`; real red-team receipt overlay passes again |

The first three used `/private/tmp/agentic-kit-compat.yvXkbz/modules/` with the candidate
kit copy. Throughline inherited `GOFLAGS=-mod=readonly`; Fathom explicitly unset it
because its subprocess contract rejects that environment variable. Logs were retained
as `new-*.assessment.*.log` in that disposable verification root.
RAC used the existing temporary module override with pinned app-kit
`d22871195401594f1083e2952d938ec5dfd655d3` and the candidate kit root.
Aggregate downstream is **not green**; no merge to `main` or release was performed.

## Intent and tool usefulness

`intent-design` and `intent-decide` kept this in existing owners under one incubation;
`intent-incident` produced the registry regression and corrected conflicting README
hook-bypass guidance. The `intent-doctor` subset checked router/living budgets, all
12 local records' membership/shape, links, compatibility/evidence sections and identical
root stanzas in the working tree (including the owner's separate, uncommitted 0007).
Router remains 80 lines and Unreleased stays under 60. Installed Claude
2.1.273 differs from tested 2.1.270; native behavior was not re-verified. Codex is 0.154.0.

Fathom was useful for exact entry points and ownership: registry/eval-gate, harness
screen/effect seams, and the existing topic judge. Broader concern searches returned
unrelated metadata and initially missed the topic gate; qualified method lookup also
returned unrelated same-name functions. Its stale-index notices and capped obligations
required targeted source checks. Final ingest found the new `AssessProfile` and
`PromptScreener.Run` symbols. This is useful navigation, not proof of enforcement;
runtime regressions and wire checks provided that evidence. No net token savings were
measured, so there is no quantitative efficiency claim.
One served stale obligation allowed `LEFTHOOK=0` for a broken hook, contradicting
current `CLAUDE.md` line 11. Current source/autonomy prevailed; stale-rule retirement
is a concrete Fathom improvement, not an instruction to bypass the gate.

Firecrawl initially lacked a working credential; primary-source browsing filled the
research gap. After the separately requested private credential update, a real official
documentation scrape succeeded. No credential or private clipboard text is in this repo.

## Next promotion evidence

A real consumer's labeled benign/adversarial cases, reviewer disagreement, complete
screening economics and a useful decision changed by this assessment. Review with 0011
on 2026-10-16 or next release. Until then these APIs stay experimental; broad catalogs,
automatic model selection and production screening quality remain deferred/unproven.
