# ADR-0001: Record architecture decisions

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** one person, many projects, long time horizons; AI
  agents as first-class readers

## Context

app-kit is the foundation for every app its author builds. Decisions made
here propagate into all downstream projects, and the person revisiting a
decision in two years is the same person who made it — with none of the
context left in their head. Additionally, AI coding agents (Claude Code and
the kit's own MCP design assistant) consume this repo as context; prose that
explains *why* is the difference between an agent that extends the
architecture and one that fights it.

## Decision

We record every architecturally significant decision as an ADR in
`docs/adr/`, numbered sequentially, using `template.md`. "Architecturally
significant" means: it constrains downstream apps, it picks a dependency the
kit will live with, or it resolves a tension between the kit's principles.

ADRs are immutable once Accepted — corrections happen by superseding, not
editing, so the decision trail stays honest. The index in `docs/adr/README.md`
is the canonical list. ADRs are written to be legible to both future-Geoff
and AI agents: dated external facts, explicit revisit triggers, no
institutional shorthand.

## Alternatives considered

- **Decisions live in VISION.md / design docs:** vision docs state intent,
  not tradeoffs; they get rewritten, which silently erases the reasoning trail.
- **Decisions live in git history / PR descriptions:** not discoverable by
  agents or by future skimming; archaeology is not documentation.
- **No formal records (solo project, "I'll remember"):** demonstrably false
  over multi-year horizons, and useless to AI assistants.

## Consequences

- Every substantial design conversation ends by either writing an ADR or
  consciously deciding it isn't architecturally significant.
- Slight ceremony overhead; mitigated by the small template.
- The MCP design assistant can serve ADRs as resources, making the kit's
  reasoning portable into every downstream project.

## Revisit when

The ADR corpus exceeds ~50 entries and needs categorization, or the kit gains
a second human contributor.
