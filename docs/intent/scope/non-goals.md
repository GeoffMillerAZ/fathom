# Scope: non-goals

Part of the scope router, `../scope.md` (the goal index, the compatibility surface and every section's member).

## Non-goals

- **No domain connectors** (PDF, Confluence, portals). Why: liability-shaped error
  costs and "the unwinnable Glean game" (`docs/design/product-vision.md` §3). Revisit
  when: an external party ships a conforming module on the published spec with zero
  connectors from us.
  - Note (2026-09-29, ADR-058): incident maps respect this non-goal. Their outward reach is a rendered link and a
    typed parameter only (a deployed SHA, a tag window); fathom never fetches, polls or stores an ops system's data,
    and the attach and connector forms are refused (`docs/design/incident-maps.md` section 6).
- **No web-scale code search, IDE language services, or embeddings-first chat.** Why:
  lanes owned by others; the harness supplies chat better and free
  (`docs/vision/01-thesis.md` §2–3). Revisit when: never.
- **No hosted multi-tenant expert backend.** Why: store rewrite, tenancy, SLA, data
  governance on a bus factor of one (`docs/vision/07` R1). Revisit when: an external
  party runs local fathom four weeks and asks in writing for a hosted view, and law 9
  hardening is proven.
- **No first-party chat or incident command center.** Why: competes with the best
  distribution channel (`07` R2). Revisit when: ≥15–20 resolved findings exist and a
  consumer asks.
  - Note (2026-09-29, ADR-058): incident maps respect this non-goal. An incident map is a committed declaration served
    read-only; drills, drill receipts, a timeline, incident status and an incident-tool export are refused
    (`docs/plans/incident-maps.md` IM-X4), and fathom never mitigates.
- **No dispatch loop.** Fathom never watches, schedules, retries, spawns, or kicks off;
  no task tracker, no workflow engine (`docs/vision/24` dispatch boundary, `09`). Progress
  views (weighted tables, a derived kanban, burn-up) are not a tracker: they read verify's
  verdicts and write nothing; no status is set by a drag, a click or an assignment (ADR-049
  decision 4). Revisit when: never.
  - Note (2026-10-16, ADR-071): readouts respect this non-goal. No readout rule, map or pack runs a command at any
    scope; a rule renders a recovery or a next step as text only (decision 7).
- **No productivity score for people.** Fathom measures work, code and agents, never ranks
  people: every per-author figure is an agent identity (model, harness, effort), and no view
  keys a row by a human (I58, ADR-051). Predictive estimates without a calibrated reading and
  auto-applied remedies are refused with it. Revisit when: never.
- **No generic token-usage dashboard.** Why: the clients and their usage tools own raw
  tokens, sessions and spend; a copy is commodity. Fathom shows cost only joined to what was
  proven (cost per proven row, ADR-049 decision 5). Revisit when: a client stops exposing its
  own usage and an adopter asks in writing.
- **No transformation or migration engine.** Why: the Moderne trap (`01` §3). Revisit when: never.
  - Note (2026-10-16, ADR-071): read as covering code and commands. A readout changes only what the agent is shown of
    a command's output; it never writes code, a file, the command or its input (the owner confirms the reading, Q-RO-3).
- **No tool joins a surface without call-log justification; no net tool growth** (law
  4; product-vision standing non-goals). Revisit when: never.
- **No grep-blocking.** Why: friction at the agent's most reflexive action is trust-fatal
  (`docs/design/coding-agent-excellence.md` §5.3). Revisit when: post-injection telemetry
  shows ≥30% of greps in indexed repos were index-answerable, and then only as a hint.
  - Note (2026-10-16, ADR-071): not fired. Bare-identifier greps were 13.5% of 29,643 greps over 30 days of local
    transcripts, an upper bound on the index-answerable share. A grep readout always shows the hits, grouped losslessly
    and capped only with exact totals and a raw recipe; an index hint beside a grep stays F75.
  - Note (2026-10-16, ADR-072): the Claude Code mods program's read-routing hint (M10) is this hint; it ships only
    as the recorded firing of the condition above (F75), and no mod answers, rewrites or blocks a grep.
- **CUE is never an authoring surface or an execution engine** (ADR-018, `09`). Revisit when: never.
- **No separate SDD binary or repo.** Why: no consumer without the index (ADR-018). Revisit when: never.
- **No Node toolchain.** Why: the artifact is a Go binary; CSS via the pinned Tailwind
  binary (`.fathom/guard.cue`). Carve-out (2026-10-16, ADR-072): `fathom plugin render` may write one
  dependency-free TypeScript hooks module, with its `claude plugin test` cases, embedded in the binary as text and
  executed only by Claude Code; it imports only Claude Code's type-only declarations, has no package manifest and
  stays within 150 lines, and fathom's build, gate and CI install, run and compile no Node, npm or TypeScript (the
  guard's `npm`, `npx`, `yarn`, `pnpm` and `bun` refusal stands). Revisit when: never; the carve-out widens only
  by a decision record.
