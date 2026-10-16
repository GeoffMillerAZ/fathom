# 0019: The kit owns the incident-evidence doctrine; an evidence engine owns the skill that binds it to its tools

**Status:** proposed
**Type:** process
**Date:** 2026-10-11

## Context

On 2026-10-11 fathom's owner-architect assessments (fathom's evidence plane for reviews,
incident prevention and incident investigation; index snapshots and a read-only environment
daemon) placed "an incident-responder skill and an evidence-before-claim doctrine" in this kit.
The assessments land in fathom v0.11.0, with fathom's design (docs/design/index-snapshots.md).
The doctrine is written here as [incident-readiness section 11](../../incident-readiness.md).
Two facts shape where the skill goes:

- fathom-* skills in this repository are generated and fathom-owned (marker at
  `.claude/skills/fathom-activate/SKILL.md:11`; source `dist/agents/skills/` in fathom). On
  2026-10-11 all nine copies here differed from fathom's source and `fathom-findings` was absent.
- The routing a responder needs (tool names, flags, the answer basis) moves with fathom's
  surface. The AGENTS.md stanza (AGENTS.md:87-118) already names tools that fathom's default
  surface no longer serves.

The owner has not ruled on the placement.

## Decision (proposed)

1. The doctrine is kit practice: read the answer basis, read the fact sheet, route
   questions to the right tool, cite a receipt for every code fact, never name a cause, never
   mitigate. It stays engine-neutral in `docs/incident-readiness.md` section 11.
2. The skill that binds the doctrine to fathom's tools is fathom-owned and generated, like
   `fathom-findings`. It cites section 11 instead of restating it.
3. The kit ships no engine-specific incident skill and no package for the fact sheet or the
   answer basis. `docs/plans/fathom.md` holds the fathom mapping as plan data.

## Rationale

The doctrine outlives any engine; the routing table does not. Keeping the binding with the
tools it names keeps both versioned together, and keeps the kit org- and engine-agnostic.

## Alternatives rejected

- **A kit-owned `incident-respond` skill naming fathom's tools** (the assessment's literal
  placement). It goes stale on every surface change, as the stanza did. Wins if: a second
  evidence engine adopts the same answer-basis and receipt contract, so the skill can bind to a
  port.
- **Doctrine in fathom only.** Other consumers of the kit's incident practice would not see
  it. Wins if: the kit retires practices §27.

## Consequences

Two homes to keep consistent: section 11 and fathom's skill. The skill cites the section by
heading, so a renamed heading breaks a link a reader can see. The vendored copy in fathom
(`third_party/agentic-kit`) carries section 11 after fathom's next `task sync-kit`.

## Preserves / forecloses

Preserves futures F19 (an incident responder over an engine that reports an answer basis), with
the binding kept outside the kit. Forecloses, while this stands, a kit-owned engine-specific
incident skill; F19 lists it under "Do not build".

## Deferred

The skill itself: FX-4 in `docs/plans/fathom.md`, with futures F19's promote condition.

## Revisit when

- The owner rules on this record (accept, reject or amend).
- A second evidence engine, or a kit-built responder agent (FX-10), needs the same binding.
- fathom asks the kit to own the skill, or fathom drops its generated skills.
