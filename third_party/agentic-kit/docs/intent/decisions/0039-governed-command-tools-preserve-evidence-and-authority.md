# 0039: Governed command tools preserve evidence and authority

Status: accepted
Type: security
Date: 2026-10-23

## Context

The owner now requests a complete portable command-tool and hook-control program,
including implementation, research, optional scanners/classifiers, economical output
and operator-controlled tuning. This extends 0035's offline-only admission. It does
not authorize installing hooks, changing this machine's configuration or bypassing
repository gates. Existing 0030-0038 and the two-incubation ceiling still apply.

Research found a concrete output-integrity failure in repository-controlled RTK
filters. It also confirmed that scanner success, local inference, device-user presence
and a policy engine's allow result each establish less than authorization of an effect.
Sources and dispositions: [research](../../research/2026-10-23-governed-command-tools.md).

## Decision

- Extend the existing W1 slot with command-local `command-guard`, initially a rooted,
  exact-content read/search composition for the kit's own synthetic review workflow.
  Reuse the existing DLP evaluator and strict Unicode checks. No new foundation,
  exported library API, runtime daemon or third incubation. Go remains the default.
- The owner-directed full target is the [delivery contract](../../designs/governed-command-tools.md)
  and [GT work order](../../plans/governed-command-tools.md), not this first increment.
  Subsequent units must satisfy their actual receiver/host and authority prerequisites.
  Keep five engineering days, zero paid inference, <=64 MiB selected corpus,
  maintainer ownership and review by 2026-11-21 or next release. Revise or park at budget.
- The first receiver is an explicit local operator invocation over independently
  supplied scope/policy and exact file hashes. It proves bounded scanning and rendering,
  not organizational authentication, secret/PII absence, global interception or a sandbox.
  Scan complete selected snapshots before any content view; any failed prerequisite
  withholds all selected content. Rule coverage is always explicit.
- Separate execution, disclosure, output projection, authority and advisory economics.
  No model, output reducer, Fathom lookup or sampled hook may clear a mandatory hold.
  Untrusted repository config cannot define trusted filters or policy floors.
- Global configuration is an operator-owned policy source, not a blanket override.
  Mandatory constraints intersect. Advisory suspension needs an exact authenticated,
  expiring lease and per-boundary validation. Lockdown narrows authority. No safety
  denial-loop release, host-wide bypass switch or self-extending maintenance window.
- Native Windows/macOS/Linux runtime probes are required for platform support claims.
  Cross-compilation proves buildability only. OS authentication/isolation remains
  host-specific. Preserve existing native contracts rather than emulate a common sandbox.
- app-kit retains presentation/accessibility ownership. Retain the documented stdlib
  CLI divergence; submit actual reusable interaction defects, not domain-policy code.
  HITLMAN handles later exact review/exception tasks without becoming an authority root.

GT5 receiver admission, 2026-10-24: the same inspection command may invoke the exact
[Gitleaks stdin qualification profile](../../designs/governed-command-tools.md#gt-c3a-gitleaks-stdin-qualification-profile)
over captured synthetic text, with verified executable snapshots, independent required
scanner scope, whole-invocation budget and bounded log/report coverage checks. A native
probe reproduced exit-0 skipped input; missing byte coverage must hold. Source bytes
count alone is insufficient. This admits the fixed adapter and scratch qualification,
not scanner installation, custom/transitive config, arbitrary execution or a new slot.

GT13a receiver admission, 2026-10-24: [CV](../../designs/command-compact.md) adds
lossless selected-row tuple encoding to the same receiver, with a seeded byte/task
baseline and ordinary JSON fallback. No cache, reducer execution, default-format
change, production economics or new slot is admitted by this profile.

GT13b receiver admission, 2026-10-24: [OR](../../designs/command-observation-reuse.md)
admits optional invocation-local reuse of exact verified zero-finding Gitleaks stdin
observations in the same file/Git receiver. Require process-start witnesses first,
private byte copies capped at 8 MiB/256 entries, exact byte comparison, aggregate
deadlines and separate actual/reused counters. Every invocation revalidates current
controls/executable/version. Native agreement and negative probes gate qualification.
No persistent cache, authority reuse, model, installation or new slot is admitted.

GT4A receiver admission, 2026-10-24: [NQ](../../designs/command-native-qualification.md)
admits a command-local fixed-roster native qualification driver over operator-trusted,
hash-pinned guard/scanner copies and owned synthetic fixtures. Bounded actual CLI
observations, independent behavioral oracles and sanitized receipts replace ad hoc
platform claims. No case skipping, installation, global configuration, model or new
slot. GT4 remains open outside actual qualified host/profile witnesses.

## Rejected alternatives

- Universal safe Bash/Git drop-in: arbitrary programs retain ambient effects. Prefer
  typed operations; opaque commands require their own controlled execution environment.
- Compress then scan, silently filter findings, or cache an allow bit: loses evidence
  or freshness. Cache exact bounded observations, then re-evaluate current authority.
- Biometric prompt alone authorizes disarming: presence is not an exact policy grant.
- LLM escalation proves absence of secrets: probabilistic review adds evidence and
  may raise concern or abstain; it cannot establish exhaustive absence.
- Standalone Luhn as a general PII/card detector (reviewed 2026-10-24): about one in
  ten uniformly distributed eligible numeric runs passes a check digit, including
  benign IDs. This cannot meet the proposed one-false-hold-per-1,000 numeric-record
  bar. A future checksum criterion needs a real, independently selected context,
  declared forms and measured false holds. Checksum validity is never authenticity.
- One central daemon or mandatory model/evidence service: increases failure coupling
  before a proven need. Keep useful dependency-free local operations.

## Revisit when

A secret or diagnostic is disclosed before checks finish; a lossy view hides a required
finding; policy drift increases authority; a suspension outlives its grant; a native
probe fails; resource or false-hold budgets are missed; or two independent consumers
demonstrate a shared extraction need. Review date and incubation budget still apply.
