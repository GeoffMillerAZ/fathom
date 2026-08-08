# Vision 18 — The four questions: instrumenting fathom's real impact on agentic coding

> Owner verdict 2026-08-08: observatory v1 is "underwhelming for the UI and
> the metrics it has available" — be the owner, ask the questions the owner
> failed to ask, instrument them, tell the story, world-class. This vision
> is that reframing; docs/plans/impact-questions.md is its binding
> promotion. Narrate is ON (14d, self-expiring) for this wave.

## 1. The reframe: activity is not impact

v1 measures ACTIVITY — calls, sessions, catches. An engineering leader
deciding whether fathom earns its place asks four different questions, and
every instrument, screen and story should answer one of them:

**Q1 — What did the agent not have to do?** (substitution)
A `find_symbol` that answers in one turn replaces a grep+read sweep. The
"with" side is directly measured. The "without" side is measured ONLY by
the counterfactual arm (law 12) — until OFF-arm data exists, the
substitution estimator renders DARK with the exact unlock command, never
an invented multiplier.

**Q2 — What did fathom prevent?** (averted defects)
Verify catches are one class. The uncounted classes: an obligation served
immediately before an edit that then conformed; a guard warning after
which the guarded action was not taken ("guard heeded" — heuristic,
labeled); a staleness tripwire before a wrong-file edit. Prevention that
is not counted is prevention that never happened, politically.

**Q3 — Where does the agent still flounder?** (friction, the flywheel's fuel)
The index's MISSES are the roadmap: queries returning empty, did_you_mean
consolations, a fathom answer followed by a grep sweep anyway (the
fallback tell). Today friction relies on an agent remembering to file an
observation; the misses must self-collect. Miss rate per tool + top
unanswered queries = the next release's backlog, auto-written.

**Q4 — Is the knowledge compounding?** (the flywheel itself)
Time-to-first-edit per session (the warm-up fathom promises to collapse),
served-rules growth, repeat-question decline, coverage of symbols by
claims/docs. Compounding is the difference between a tool and a platform.

## 2. The instruments this demands (what v1 lacked)

- **Call outcomes in the call log** (Q1/Q3): every logged tool call gains
  result_count and matched_via — a hit, a did_you_mean consolation, or an
  empty miss become distinguishable, fleet-wide, forever.
- **Session shape in the ledger** (Q4): first_edit_turn — how many turns
  a session spent in the dark before it started changing code.
- **Rules follow-through** (Q2): injected_rules_used already lands; the
  ledger's story is "served AND the edit conformed" — v1 counts served,
  the conformity checker is a named next wave (IQ-13).
- **The miss ledger** (Q3): misses aggregate per service and per tool in
  the observatory; the top-misses table IS the auto-written backlog.

## 3. The story layer (nobody reads JSONL)

- **Session story** (/observatory/session?id=S): ONE session as a
  timeline — context injected, questions asked and answered (or missed),
  edits, the catch at the gate. viz.Timeline. This is the demo page; a
  single honest session replay sells harder than any aggregate.
- **The week** (/observatory/story): the trailing 7 days composed from
  ledger + catches + misses + observations: top wins with their plan
  rows, new friction filed, coverage delta. Template-composed always;
  LLM-narrated when the daemon has a model configured (the compile layer
  already does) — and ALWAYS labeled a lens (law 14).
- **The landing page answers the four questions** in one glance, each
  with its number and its screen.

## 4. What stays sacred

Law 3 (absence is named, never zero), law 12 (no savings without arms —
the estimator stays dark until OFF-arm sessions exist), law 13 (all of
this is loopback; the brief is the only shareable artifact and a human
sends it), law 14 (stories are lenses). The credibility of every number
on these pages is the product.

## 5. Enhancing the tool itself (beyond measurement)

- **Auto-narrate on onboarding**: `fathom init` arms a bounded narrate
  window (self-expiring, loudly announced) — the interesting data is at
  the START of an adoption, exactly when nobody remembers to arm it.
- **Duration UX**: `--for 14d` should parse (days are how humans speak).
- **Named next waves, each behind a trigger**: the session-start context
  pack (inject repo brief + open plan rows at SessionStart — collapse
  time-to-context directly); the obligation-conformity checker; the LLM
  weekly narrative. Deferred rows with triggers, not vaporware.
