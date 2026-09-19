# 0012: Compose domain assessments without a universal score

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-16
**Extends:** 0011; catalog ownership in D13/CAT is unchanged.

## Context

The owner wants evidence-based selection and improvement across agentic engineering,
including red teaming and configurable prompt screening. Existing catalogs, fitness
functions, evaluator and provenance primitives have different responsibilities.
Combining them into one authoritative catalog or ranking would erase those distinctions.

## Decision

- Share a versioned assessment lifecycle: applicability, criterion definition,
  subject identity, observations, scoped findings, decision and re-evaluation trigger.
  Domain collectors, rubrics and thresholds remain domain-owned. No universal score.
- Extend experimental `fitness-function-registry` with assessment profiles and
  evidence-linked findings. Keep `eval-gate` as the weighted-arithmetic implementation;
  do not create another evaluator, catalog, provenance store or policy platform.
- Required failures and unknowns block soft optimization. Non-applicability needs
  explicit profile permission, reason and evidence; an entirely inapplicable profile
  cannot pass vacuously. Stale or mismatched observations remain unknown.
- Separate deterministic checks, empirical measurements, heuristics and human review.
  Caller-declared evidence is not authenticated or independently verified by hashing.
  Retain material inputs and artifacts in consumer-owned storage for reconstruction.
- Add an explicit small-model prompt-screening composition against a versioned local
  refusal policy. Classification is fallible evidence, not authorization. Deterministic
  policy decides allow/refuse/review; ambiguity and classifier failure hold execution
  for review by default. No tools, protected output or speculative effects before a
  required screen completes. Provider choice and accessibility remain caller-owned.
- Defaults cover attempted unauthorized action, protected-data extraction and instruction
  hijacking, not a purported universal legal or moral standard. Benign discussion and
  authorized security analysis are controls against unnecessary refusals. Organizations
  explicitly add their content standards and validate thresholds against labeled cases.
- Extend 0011's existing incubation, not the active-incubation count. First profiles
  assess retained benchmark evidence and an offline security entry point with actual
  handler counters and benign controls. A profile library for every domain is deferred.

## Rationale

The reusable product is a short, inspectable path from a requirement to its evidence
and a policy outcome. Common envelopes reduce integration cost; common scoring across
unlike domains conceals risk. Design, research, defaults and acceptance:
[engineering assessments](../../designs/engineering-assessments.md).

## Alternatives rejected

- Replacing all catalogs with an assessment service: unnecessary migration and ownership.
- Averaging safety, cost and quality into an excellence score: hides disqualifying failures.
- Trusting a classifier's confidence or an injection alert as an authorization decision.
- Automatically enabling an unbenchmarked small model, or treating synthetic tests as
  demonstrated live screening quality.

## Revisit when

- 2026-10-16 or next release, together with 0011: promote only with consumer evidence
  of adoption cost, actual failures and useful decisions; revise or park otherwise.
- Two domain collectors cannot use the profile without duplicating evaluation logic.
- Required screening permits an effect before its decision, or default refusals block
  a labeled benign case without a documented policy reason.
- A consumer needs authenticated evidence or organization-wide policy distribution;
  record those trust and migration requirements before adding infrastructure.
