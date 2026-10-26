# 0033: Decision-site utilities extend the offline reference

Status: accepted
Type: architecture
Date: 2026-10-23

Request: "look for ways that decision models like openai decisions api can enhance and uplift use cases in the agentic kit and any of our projects and then help to make utlities and guidance on it"

## Context

0017/0018 require optional calibrated judgments; 0026 forbids model-only authority.
0018's review trigger fired: current official OpenAI documentation describes general
availability, shared-prefix caching and expanded native inputs/question counts. These
facts do not establish application calibration or justify a silent native-feature mapping.
Review Kit and Fathom already use bounded judgments; retrieval-label and clarification
pilots have concrete evidence seams. Their domains retain authority and adoption.

The port already supplies adapters, replay/cache, exact request keys and readings.
The missing W1 reference path is practical site preparation, captured-response inspection
and independently scoped labeled studies. Source-backed opportunities and rejections
are recorded in the [decision-site guide](../../decision-sites.md).

## Decision

Extend W1's existing second-slot offline reference under 0032 with the bounded
[DL-1..6 specification](../../designs/decision-lab.md). Add `decision-lab`, fixture recipes,
guidance and a narrowly routed adoption skill. The CLI does not dispatch providers or
authorize effects. Initial utility scope is text-only string-choice packs, with explicit
unknown outcomes, cohort/identity binding, independent labels, refused/missing cases
and per-predicted-class readings. Preserve the existing port for other question shapes.

Kit maintainer owns the same W1 bounds: five engineering days, zero provider spend,
<=64 MiB corpus and review 2026-11-21 or next release. No third implementation slot,
new shared foundation, dependency, gate edit, consumer migration or global wiring.
No full AE completion, live-quality, savings, provider parity or production claim.
Update dated API guidance without rewriting historical provider observations.

## Rejected alternatives

- Another generic router or judgment runtime: existing primitives suffice. Wins if:
  two consumers demonstrate a shared missing contract that local composition cannot cover.
- Make a model compute exact scope, tests, dates, permissions or scanner results:
  loses computable guarantees. Wins if: never for a required deterministic property.
- Treat confidence, cached answers or frontier labels as calibrated truth: no independent
  evidence. Wins if: a site-specific independently adjudicated study meets declared bars.
- Automatically migrate existing Review Kit/Fathom lanes or enable paid providers:
  their owners and measured acceptance govern adoption. Wins if: a bounded exact upgrade
  and authorized campaign establish value within the receiving project's policy.
- Claim the lab supports every native input or predicate/score calibration: a choice
  study cannot supply their domain oracle. Wins if: separately specified conformance
  and evaluation fixtures cover the particular added capability.

## Revisit when

Review date arrives; a malformed or missing result becomes usable; context changes
reuse a reading; setup exceeds 15 minutes; two consumers need the same missing seam;
or a bounded live study establishes benefit. Park expansion without new evidence and
admission. Retirement, tier changes and consumer/global changes retain existing rules.
