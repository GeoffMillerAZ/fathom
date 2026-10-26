# 0030: Quality boundaries compose hard rules and calibrated holds

Status: accepted
Type: architecture
Date: 2026-10-22

## Context

The owner requests reusable decision-model checks at Git/Lefthook and other project
boundaries, with guidance and an adoption skill. Decisions 0017/0018 require optional,
site-calibrated judgments. Decision 0026 rejects model-only authority or completeness.
No evidence has fired that rejection's revisit trigger. F18's lost-pin engine remains
unpromoted. Existing `agent-hooks` fail-open economic guidance is a different contract.

## Decision

Admit one bounded, command-local composition, `cmd/quality-gate`, under 0004/0025.
Subject: the kit maintainer's repository boundary workflow. Baseline: existing hard
gates plus manual review. Owner: kit maintainer. Budget: this delivery and offline
fixtures, zero paid runs, no new library root, dependency, daemon, or consumer mutation.
Review by 2026-11-21 or the next release, whichever occurs first. This reference takes
one implementation slot while older incubations retain maintenance-only disposition.
Exit: actual staged/committed/ref-update negatives and Lefthook invocation reject known
violations, with explicit unknowns and no model permission to erase deterministic failure.

Hard rules inspect bounded immutable Git objects. Enforcement reads a trusted committed
policy, binds its digest/commit and exact trees, and rechecks moving inputs before return.
Unsupported/missing required inputs are unknown and nonzero. Existing tests, linters,
scanners and domain gates stay separate. No arbitrary repository command is executed.

Reuse `foundations/decision` for optional OpenAI Decisions and TypeSafe/Jev (direct or
OpenRouter). Explicit invocation permission controls disclosure. Environment model pins
bind resolved identity. Shadow is default. A hold site clears only with exact-site,
question, provider, model and freshness-bound calibration with independent labels and
declared class coverage. Provider errors, unknowns and missing calibration retain holds.
One bounded batch per snapshot has no automatic retries or fallback. Shadow-only exact
TTL caching never authorizes a required hold. Reports omit source and credential values.

Local hooks are developer feedback, bypassable and non-atomic with commit. Required CI
uses a separately trusted pinned binary and base policy against the proposed commit.
Protected-branch configuration and consumer adoption need their own observed proof.
The scaffold installs no hooks and changes no global configuration. No kit gate weakens.

## Rejected alternatives

- Model confidence as deterministic correctness or a replacement for tests/security
  scanning: violates 0026. Wins if: independent proof for the exact property, not a score.
- Automatic model enforcement on every commit: disclosure, latency and false holds lack
  site evidence. Wins if: a named consumer proves calibrated benefit within its budget.
- Persistent cached permission, generic waivers, promise/lost-pin ledger or automatic
  policy repair: beyond this requested boundary and authority. Wins if: separately
  admitted contracts prove freshness, authenticity and domain authority; F18 still applies.
- New framework/foundation: existing typed decisions suffice. Wins if: two consumers
  demonstrate a missing shared contract that command-local composition cannot supply.
- Import app-kit's styled terminal stack: conflicts with I1's zero dependencies. Keep
  its 0/1/2 exit convention, plain output and typed JSON. Wins if: I1 is explicitly changed.

## Revisit when

Review deadline arrives; two consumers need a shared library; a real labeled study
measures benefit or false-clear harm; hooks skip an expected invocation; Git/provider
contracts drift; or snapshot/egress/calibration identity can be bypassed. Promote only
after consumer acceptance; revise/retire if maintenance or review burden exceeds benefit.
