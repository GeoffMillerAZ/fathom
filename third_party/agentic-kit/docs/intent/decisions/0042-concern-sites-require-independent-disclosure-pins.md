# 0042: Concern sites require independent disclosure pins

Status: accepted
Type: security
Date: 2026-10-24

## Context

The requested command replacement program includes optional local classifiers and
stronger judgment. Existing decision/LLM ports cover the protocol. GT12 still lacks
a selected native runtime/model, measured residency/egress/logging and calibrated
quality. A local HTTP address can front cloud inference. Model content is untrusted.

## Decision

Admit [MC](../../designs/command-concern-classifier.md)'s file-inspection receiver
inside 0039's existing W1 slot, budget and review date. Reuse the existing decision
and OpenAI-compatible LLM adapters. No new foundation, dependency, engine or slot.

The optional concern site runs after mandatory regex/scanner checks and before any
source projection. Concern, uncertainty, malformed/missing evidence and failures
withhold every source row. No-concern adds no hold; it cannot erase a mandatory one.
Default absent preserves the existing behavior and wire. Git model use holds until
its separate path is implemented. No installed hooks or global configuration change.

Compile the experimental client only with `-tags command_classifier`. The ordinary
build rejects every selected classifier control as unsupported before source/scanner
work. Help identifies the compiled capability. Qualify and pin the two artifacts
separately. Initial eager linking increased default-mode median time by 67.46% on
MC-A10's declared corpus, exceeding its 25% bar. Retain that failed observation;
rerun the unchanged corpus against the lightweight build. The tagged binary carries
the HTTP/model startup cost and has no default-mode performance qualification.

This first profile permits only independently selected public/synthetic byte hashes
from protected scope. Scope also pins the exact policy-file SHA-256, including its
transmitted criterion and endpoint selection. Decode and hash the same captured policy
bytes; use that immutable value through dispatch. Both control files remain operator
owned; this CLI does not authenticate their issuer or protection. Check requested
pins before any source read/scanner call, then captured bytes before a model request.
Candidate declarations, filenames and model assurances cannot establish eligibility.
Private inputs require a later qualified deployment boundary.

The transport fixes one numeric loopback destination, no proxy/redirect/fallback,
one request and finite input/wire/reply/token/time bounds. Pin requested and expected
response model names independently; treat the response name as an observed assertion,
not authenticated identity. Preserve only fixed outcomes and bounded counts publicly.
No raw answer, model/endpoint string, probability, hidden reasoning or error body.

The receiver is executable, not a live model qualification. Development uses owned
synthetic HTTP peers only. GT12 remains open for native model/runtime, source policy,
cohort/calibration, resource fit and declared benefit. No paid or existing local model
call is admitted as incidental implementation testing. Research: [MC sources](../../designs/command-concern-classifier.md#sources).

## Rejected alternatives

- A separate invocation lab as receiver completion: cannot prove withholding at the
  actual output boundary. Wins if: an independently named receiver needs that tool.
- Loopback or a cloud-disabled setting alone proves residency: neither observes all
  runtime/egress/logging behavior. Wins if: never as a standalone proof.
- Uncalibrated confidence clears a required hold: confidence is not authorization or
  measured accuracy. Wins if: never; calibrated assistance still retains hard floors.
- Private-data use disguised by synthetic tests: tests qualify their fixture scope.
  Wins if: never. Admit actual private disclosure only with its independent evidence.
- Universal temperature-zero rewriting as a determinism guarantee: sampling settings
  are model-specific and do not remove uncertainty. Wins if: an exact native profile
  needs explicit-zero support and its typed adapter/quality bar are qualified.
- Eagerly link the optional client into every guard: measured startup failed MC-A10.
  Wins if: the combined artifact meets the declared performance bar, or a named
  consumer explicitly accepts its measured cost. No extra process or handwritten
  HTTP protocol replaces the existing adapter merely to reduce binary size.

## Revisit when

Any source leaves without its disclosure pin; a required hold is cleared; request,
reply or attempt bounds fail; endpoint/model identity drifts; false holds exceed the
selected receiver's bar; a native deployment supplies the missing evidence; or two
independent consumers demonstrate a shared adapter seam. Review 2026-11-21/next release.
