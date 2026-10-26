# 0032: Admit offline practice review and local diagnostics

Status: accepted
Type: architecture
Date: 2026-10-23

Request: "lets do another uplift and quality and gap closing and creature comfort features pass"

## Context

0025 sequences practice/data reference W1 before broader runtime expansion. 0030 and
0031 already admit quality-gate; local configuration and coverage are difficult to
inspect without attempting enforcement. A source-backed audit reproduced a text report
that counted, but did not identify, changed paths outside selectors. Independent scope
is designed but not yet demonstrated by the W1 entrypoint. No F5/F18 promotion or new
shared-engine trigger has fired. Older expansion stays maintenance-only under 0025.

## Decision

Implement the [bounded specification](../../designs/uplift-2026-10-23.md), UP-1..6.
Extend the existing quality command with offline configuration-only diagnostics and
bounded actionable coverage output. Keep its runtime, policy, environment and source
trust boundaries explicit. A doctor cannot attest provider operation or installed hooks.

Admit W1's first synthetic reference as the second implementation slot under 0004:
reuse Catalog Kit exact binding and the existing assessment function in an example.
Kit maintainer owns it; two synthetic subjects, independent trusted expected scope,
three bounded deterministic property checks, zero model spend, five engineering days,
at most 64 MiB fixtures (1 MiB per input), review 2026-11-21 or next release. Baseline
is assessment over only loaded criteria. The reference must catch planted omitted
subjects/criteria and reach a useful guided offline outcome within 15 minutes.

This is partial implementation of AE-1/AE-2, not their full catalog/domain coverage,
Review Kit exchange, lifecycle or adoption acceptance. Keep those whole rows open.
No new foundation, external dependency, global hooks, consumer migration or gate edit.
Diagnostics retain app-kit's plain-output/0-1-2 conventions; 0030's dependency-free
CLI divergence remains necessary under I1.

## Rejected alternatives

- Another universal reviewer/policy engine: existing contracts suffice. Wins if: two
  consumers demonstrate a shared missing seam that their local adapters cannot cover.
- Generate expected scope from criteria/results: makes omissions invisible. Wins if:
  never for independent completeness claims.
- Doctor silently repairs policy, calls providers or certifies hook/CI adoption:
  exceeds read-only diagnosis and cannot establish invocation. Wins if: a separately
  authorized, bounded action contract proves its effects and recovery.
- Close whole AE rows because a narrow example passes: conflates integration scope
  and adoption. Wins if: every owning acceptance exit actually passes.

## Revisit when

Review date arrives; a planted omission passes; two consumers need the same missing
contract; setup exceeds 15 minutes; diagnosis exposes a secret or is mistaken for
enforcement; or maintenance exceeds the bounded budget. Park expansion without new
evidence and explicit admission; this record authorizes no tier demotion or retirement.
