# ADR-0088: AGENTS.md alone carries the adoption router

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-29
- **Extends:** ADR-0024 (agent routing only); the `agents` field, block text and `adoption router` output are unchanged

## Context

ADR-0024 made `agents: true` require the same marked router in CLAUDE.md and AGENTS.md.
On 2026-09-29 the owner decided that AGENTS.md is the single instruction file in these
repos and CLAUDE.md is not maintained; fathom, agentic-kit and app-kit carry AGENTS.md only.
Fathom's own adoption check then failed `router:CLAUDE.md` ("Required file is missing")
although AGENTS.md held the exact router, so a checked consumer had to recreate a file its
owner had retired.

Dated host facts, taken from fathom ADR-059 and not re-verified here: Claude Code 2.1.277
and later, in its default mode, reads AGENTS.md itself only when no CLAUDE.md is found;
when one exists it is loaded instead, so a CLAUDE.md without an `@AGENTS.md` import hides
AGENTS.md. Codex reads AGENTS.md. An installed version does not prove a host loaded
anything (adoption.md).

## Decision

With `agents: true`, AGENTS.md must contain the canonical block exactly once; a missing,
duplicated or drifted block fails `router:AGENTS.md`. AGENTS.md alone satisfies the router
requirement.

CLAUDE.md is optional. Absent, `router:CLAUDE.md` is not-applicable and the file is not
listed as inspected. Present, it passes when it links to AGENTS.md (a symlink whose target
is exactly `AGENTS.md`), imports it (a whitespace-delimited `@AGENTS.md` or `@./AGENTS.md`
token outside fenced code and inline code spans), or still carries the canonical block
itself, the ADR-0024 pair. Otherwise it fails and the remedy names the import.

Only the root CLAUDE.md is examined. No other symlink is followed; no other import
spelling is counted, so a near miss fails visibly. The check stays static: nothing is
written and no host-load or invocation is claimed.

## Alternatives considered

- **CLAUDE.md must import AGENTS.md or be absent.** Matches the owner's end state, but a
  consumer whose paired block passes today would start failing with no change in what
  Claude reads; exit codes are on the compatibility surface. Wins once no known consumer
  keeps a paired block.
- **Stop checking CLAUDE.md.** Simplest, but passes a CLAUDE.md that shadows AGENTS.md:
  the false pass ADR-0024 names as a revisit trigger.
- **A new selection field for instruction files.** No consumer needs it; `agents` already
  says "route agents here" and a schema bump would ripple through drafts and pins.
- **Parse Claude's full memory grammar and follow imports.** Unbounded, and a wrong parse
  is a false pass.

## Consequences

- Fathom and any AGENTS.md-only consumer pass without a CLAUDE.md; no check that passed
  before fails now. A paired consumer's report is unchanged, including finding order.
- An import-only CLAUDE.md passes, so consumers can keep the file without a second block.
- A consumer whose host reads CLAUDE.md alone and has none passes statically; that is the
  ADR-0024 limit (text parity is not host execution). A `.claude/CLAUDE.md` is unchecked.
- Docs and the `draft --agents` help text change; rebuild the CLI to pick up the check.

## Revisit when

- A supported host reads CLAUDE.md but not AGENTS.md, or Claude Code changes how it loads
  AGENTS.md or spells imports: an AGENTS.md-only pass becomes a false pass.
- No known consumer keeps a paired block: require the import or absence.
- A consumer keeps its import in `.claude/CLAUDE.md` or CLAUDE.local.md and is refused.
