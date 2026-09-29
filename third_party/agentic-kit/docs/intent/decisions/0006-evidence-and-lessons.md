# 0006: Prose and shared lessons retain verifiable evidence and scoped authority

**Status:** accepted
**Type:** domain
**Date:** 2026-09-12

## Context

The owner names precise prose traceability and cross-workload learning as core outcomes.
`governed-prose-kit.parseNodes` currently copies all input references to every line;
this is not claim-level support. Futures F3 also made shared learning depend on F2's
central memory, even though consumers may correctly own separate memory stores.

## Decision

- A factual prose unit identifies its claims and the specific evidence supporting each
  claim. Paragraphs may contain several claims; attaching a bibliography to a paragraph
  does not establish coverage. Non-factual text is explicitly classified, with review
  of that classification so assertions cannot escape the gate as decorative prose.
- Evidence refers to a versioned source snapshot and an exact locator; source access
  and retention remain governed. Preserve source, extracted fact, inference, and generated
  wording as distinct objects with author and transformation provenance.
- Report three separate properties: reference integrity (deterministic), semantic
  support (evaluated or reviewed), and source truth/authority (domain dependent).
  Provenance records supplied inputs and declared support, not a model's hidden causal
  reasoning. No claim of deterministic factual truth follows from a valid link.
- Corrections invalidate dependent claims, prose, and approvals. Publication evaluates
  the current artifact and evidence versions. Caller-provided IDs alone are insufficient.
- A shared lesson packages a failure scenario, source/provenance, applicable context,
  proposed remedy, independent regression check, privacy classification, version, and
  supersession/revocation condition. It starts as a candidate.
- Promotion requires review and a consumer rerunning the check. A receiving workload
  imports a versioned lesson; receipt of a lesson never grants execution authority or
  silently changes local policy. Share redacted reproductions, not raw private histories.
- Memory stores remain consumer owned. The kit supplies contracts, validators, replay
  scenarios, and adapters; Git and existing Fathom distribution may carry artifacts.
  An autonomous global memory database is not a prerequisite for shared learning.

## Consequences

Prose G6 and lesson transfer G7 share the existing evidence/provenance spine. Their
composition is planned, not shipped by this record. They can be built independently.
Detailed acceptance scenarios live in [the workflow design](../../designs/governed-workflows.md).

## Alternatives rejected

- Assign every available source to every sentence: conceals unsupported claims.
- Treat an LLM judge or source hash as proof of truth: confuses distinct guarantees.
- Synchronize every agent's memory automatically: spreads private data and unreviewed
  conclusions, and couples otherwise independent workloads.

## Revisit when

- A pilot cannot represent its evidence without losing sentence/claim-level support.
- A shared lesson is harmful outside its originating context or leaks sensitive data.
- Two consumers need distribution guarantees that versioned files cannot provide.
