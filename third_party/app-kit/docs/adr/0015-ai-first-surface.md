# ADR-0015: Treat agents as first-class users of the kit

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** the kit's only user is a developer+agent pair
  (VISION.md §2); the ambient-knowledge acceptance test (VISION.md §North-star
  #4); multi-decade horizon — future-Geoff arrives with no context in his
  head, exactly like a model does; legibility decays silently unless something
  enforces it

## Context

VISION.md §2 already answers "who is the user": every app from here on is
built by Geoff working with AI agents, and machine legibility is the
second-most-important property of the kit after correctness. This ADR turns
that principle into commitments concrete enough to be checked in CI and cited
in review.

What we know about how agents actually consume development surfaces, dated:

- **Context is the scarce resource.** A documented production deployment
  burned 143k of a 200k-token window on MCP tool schemas alone — 72% of
  context gone before the first user query
  (https://agentmarketcap.ai/blog/2026/04/08/mcp-context-bloat-enterprise-scale-tool-definitions-agent-context-budget,
  2026-04-08). Surfaces that dump everything lose to surfaces that answer
  precisely.
- **The successful tool shapes have converged.** shadcn/ui, Figma Dev Mode,
  Storybook, and the TailwindPlus server we run today all settled on
  names-first, search-then-get, full content only by exact identifier
  (https://ui.shadcn.com/docs/mcp,
  https://developers.figma.com/docs/figma-mcp-server/tools-and-prompts/,
  https://storybook.js.org/docs/ai/mcp/overview). D11 adopts this shape for
  the Assistant.
- **Per-project agent config is host-specific.** Correction, 2026-09-21:
  Claude Code's project `.mcp.json` is not Codex registration; Codex uses
  `config.toml`. Keep paired instruction routers but verify each host separately
  using [agent setup](../design/agent-setup.md).
- **Our own machinery is checkable.** CUE v0.16.1 (2026-04-08) gives `cue vet`
  and committed `gengotypes` output (D7); templ v0.3.1020 (2026-05-10) added a
  `-check` flag for validating templates in CI without regenerating
  (https://github.com/a-h/templ/releases/tag/v0.3.1020). Machine-readable
  surfaces can therefore be machine-*verified* surfaces.

And one structural fact about this project: the human has agents. Geoff does
not scroll catalogs, grep conventions out of prose, or hand-walk upgrades —
agents do, on his behalf. A surface optimized for human browsing but opaque to
models slows down every single task.

## Decision

**Agents are first-class users of the kit.** Not "the docs are also readable
by AI" — the agent is a primary audience whose needs are designed for,
budgeted for, and tested. The concrete commitments:

1. **Every schema is machine-readable.** All manifests, config, tokens, and
   component/pattern metadata are CUE (D7). No load-bearing convention exists
   only in prose; if an agent must conform to it, an agent can validate
   against it.
2. **All kit knowledge is queryable.** Components, patterns, tokens, ADRs,
   and upgrade notes are served by the Assistant (D11) with search-then-get
   token discipline. Knowledge that cannot be queried mid-task effectively
   does not exist.
3. **ADRs are written for mid-task model consumption.** Dated, versioned
   external facts with URLs; explicit revisit triggers; honest alternatives;
   no tribal shorthand ("the usual approach", "as discussed") that assumes a
   memory the reader does not have. `docs/DECISIONS.md` is the compressed
   register an agent loads first; ADRs are the expansion it fetches on
   demand.
4. **Every scaffolded app ships an agent briefing.** `appkit new` emits a
   CLAUDE.md pack (kit version, the Loopback Invariant stated, the app's
   conventions, the failure signatures worth knowing — e.g. the GOPRIVATE
   miss from D13) and a committed `.mcp.json` connecting the Assistant. A
   fresh agent session in a fresh checkout is productive without a human
   priming it.
5. **Scaffold output is agent-legible by construction.** Predictable layout —
   the mirror-in-miniature of
   [../design/repo-layout.md](../design/repo-layout.md) — with named
   conventions and one obvious place for everything. An agent that has seen
   one kit app has seen them all.
6. **Gallery pages are deep-linkable.** Every component, token set, and
   pattern demo has a stable URL (D12), so the Assistant and agents can cite
   "the kit way" with a link that renders, not a description that drifts.
7. **The tiebreaker rule:** when a design choice trades human convenience
   against agent legibility, agent legibility wins. The human has agents; the
   agents do not have the human's memory. (Example: explicit, slightly
   verbose CUE field names beat clever short ones; numbered ADR files beat a
   wiki with friendly titles.)

Boundaries: this ADR commits to surfaces, not vendors — CLAUDE.md and
`.mcp.json` are today's concrete carriers (2026-06-10), and the commitment is
"a committed agent briefing and a committed Assistant connection", whatever
the file names become. It does not mandate prompt formats, agent products, or
how Geoff runs his sessions.

## Alternatives considered

- **Human-first docs, agent layer bolted on later.** Attractive: write
  naturally now; models are good readers, surely they'll cope. Lost because
  the retrofit is a rewrite: undated facts, tribal shorthand, and prose-only
  conventions are exactly the failures a model cannot repair from context —
  it will confidently act on stale versions and invented conventions. Drift
  between prose and reality is invisible until an agent ships a bug built on
  it.
- **"The code is the documentation."** Attractive: code never lies, agents
  read code fluently, zero writing cost. Lost because code is the residue of
  decisions (VISION.md §1) — the alternatives, the costs accepted, and the
  revisit triggers are not in the code. An agent reading only code
  re-litigates settled decisions or cargo-cults them; both are slow and both
  are wrong. This is the precise failure mode ADR-0001 exists to prevent.
- **RAG over a freeform wiki** (write anything anywhere; embeddings find it).
  Attractive: zero structure tax, fashionable, and we already have an
  embedding pipeline (D10). Lost because retrieval over unstructured prose
  returns approximations, and schemas need validation, not vibes: an agent
  must be able to `cue vet` its output, not hope a similar paragraph was
  retrieved. Semantic search remains one retrieval channel *inside* the
  Assistant — it is not the substrate.
- **No explicit policy.** Attractive: zero ceremony; we mostly believe this
  stuff anyway. Lost because the default gradient always points toward human
  convenience — every undated fact and unqueryable convention is individually
  harmless, and the kit fails the ambient-knowledge test by a thousand cuts.
  The tiebreaker rule exists to be cited in review when the gradient pulls.

## Consequences

- **The docs are the API, so doc rot is platform rot.** This is the honest
  price of the decision, and it gets CI teeth rather than good intentions:
  - link checking over the docs corpus (every cross-reference in
    DECISIONS.md, ADRs, and design docs must resolve);
  - regeneration cleanliness — `templ generate -check` (or generate+diff),
    `cue vet`, committed `gengotypes` output diffed against fresh generation
    (D4, D7);
  - upgrade-note presence: a minor tag without `docs/upgrades/vX.Y.md` fails
    the release (D13);
  - scaffold tests: `appkit new` output must contain a valid `.mcp.json`,
    the CLAUDE.md pack, and an `app.cue` that passes `cue vet`.
  The CI outline lives in [../design/repo-layout.md](../design/repo-layout.md).
- Easier: the ambient-knowledge test (VISION.md §North-star #4) becomes
  mechanically checkable — fresh repo, Assistant connected, canonical
  questions, assert the answers cite real patterns and live Gallery URLs.
- Easier: onboarding cost for any new agent session drops to "read CLAUDE.md,
  query the Assistant" — which is also the onboarding cost for future-Geoff.
- Harder: every architecturally significant decision now costs an ADR-shaped
  writeup with dated facts and URLs. Accepted: the writing *is* the product
  (decisions are the unit of reuse, VISION.md §1).
- Harder: human-facing niceties (narrative tutorials, pretty doc sites) are
  deprioritized when they compete with machine surfaces. Accepted under the
  tiebreaker rule.
- Risk: the CLAUDE.md pack and `.mcp.json` conventions are 2026 artifacts of
  specific agent products and will shift. Tripwire: a major agent tool we use
  changes its config convention → pack update in the next minor, upgrade note
  included.
- Risk: doc checks decay into noise (flaky external links, false-positive
  diffs) and get ignored. Tripwire: first `--no-verify`-style bypass of a doc
  check → fix the check the same week (external links move to a pinned
  allowlist; internal links stay hard failures).
- Risk: over-documentation slows decisions. Mitigated by ADR-0001's
  significance bar — not everything is an ADR; everything load-bearing is.

## Revisit when

- Agent clients consume MCP resources and long context reliably enough that
  some tool-mirroring or compression discipline becomes redundant weight
  (same trigger as ADR-0011's resources-vs-tools revisit).
- The CLAUDE.md / `.mcp.json` conventions are superseded by an
  industry-standard agent-config format — migrate the pack, keep the
  commitment.
- A second human contributor joins — the tiebreaker rule needs renegotiation
  when there are humans without agents in the loop.
- An agent fails the ambient-knowledge test *despite* conforming surfaces —
  the commitments above are wrong or incomplete, and this ADR needs
  supersession, not patching.
