# 32 — The decision port: typed fast decisions behind a flag

Status: ACCEPTED 2026-09-22 (ADR-042, owner-directed after the assessment "Jev and
the Strengths Bench"). Plan: `docs/plans/decision-port.md`. Default: off.

## 1. The question

Fathom is a typed, evidence-tiered system. Its facts come from the index and are
never guessed. But it still makes a handful of fuzzy decisions, and today it
makes them either with heuristics or with a slow text model:

| decision | made today by | cost of being wrong |
|---|---|---|
| which six of ~140 eligible rules to serve at an edit | scope tier, then lexical relevance | the governing rule is displaced (O23, O54, O57) |
| which verb an issue-shaped prompt wants first | a stanza and a brief | the surface is never used (O102: `localize` in 0 of 115 cells) |
| which guide sections carry a binding rule at all | compile every section with a text model: 61 sections, 5.1M tokens, $2.79, 75 min per subject and commit | the compile is the slowest and dearest step of every wired session and bench cell |

Each of these is a typed question about state fathom already holds: *does this rule
govern this edit* (yes/no with a probability), *which of these verbs* (a choice with a
probability per option), *is this section a rule* (yes/no). A class of model now
exists that answers exactly that shape in tens of milliseconds for a fraction of a
cent — TypeSafe AI's Jev, a "System One Model": typed questions in, calibrated
probabilities out, never text. It is in early access. Whether it is Jev, a small
local classifier, or an ordinary LLM asked for JSON, the *shape* is the same and the
value is in the seam, not the vendor.

## 2. Decision

A **decision port** — `foundations/decision` in agentic-kit — with three question
kinds mirroring the vendor primitives 1:1 (`choice`, `score`, `noul` = a yes/no
probability), one batched request per call, and a `confidence` derived from the
probability spread by the same formula the vendor documents. Two adapters ship now:
`none` (returns a typed "no provider" error) and `llm` (any `llmclient` asked for a
JSON object against the question schema — Haiku through the existing lanes). A
`typesafe` adapter is a documented future (`futures.md` F21), added when access
exists and an A/B earns it.

Fathom consumes the port through `.fathom/config.toml`:

```toml
[decide]
provider           = "none"   # none | llm      (default none: no call is ever made)
profile            = ""       # llm: the [llm] profile lane to use (default: the compile lane)
compile_threshold  = 0.5      # noul below this → the section is not compiled
route_confidence   = 0.9      # choice confidence at/above this → one firm route line
rank_within_tier   = true     # obligations: re-rank inside a governing-scope tier only
```

Three sites, each with its deterministic path intact:

1. **Compile pre-filter.** Before a section is sent to the compile model, one batched
   `noul` per section asks whether it states a binding rule for code in this
   repository. Sections below `compile_threshold` are not compiled. They are counted
   (`sections_prefiltered`, an additive `compile_runs` column), disclosed in the
   `layer_coverage` envelope as `prefilter_skipped`, and — this is the invariant —
   treated as *uncompiled* by ADR-037's raw fallback, so a skipped section that does
   govern a path is still served raw, never silenced. `fathom compile --continue`
   compiles them later on request.
2. **Obligations ranking, within a tier.** ADR-035 ranks by governing scope: an
   exact-file rule outranks a package rule outranks a root rule, and no model may
   change that. Inside one tier, a batched `noul` per eligible rule ("does this rule
   govern this edit") re-orders the rules before the `MaxObligations` cap. The
   envelope says `ranked_by: "none" | "llm"` and carries each served rule's
   `relevance`. Displaced counts, budgets and the ≤6 cap (I35) are unchanged.
3. **Prompt routing.** `fathom prompt-hook` asks the daemon (`POST /route`, loopback)
   for the verb an issue-shaped prompt wants; the daemon asks the port one `choice`
   over the verb set. At or above `route_confidence` the hook injects one route line;
   in the confirm band it injects a hedged line; below the unsure band it says
   nothing. The hook never calls a provider itself (I14), draws on its existing shared
   deadline, and fails open.

## 3. What never changes

- **Facts stay facts.** The port ranks, routes and triages. It never asserts that a
  symbol exists, a caller calls, a citation holds, or a rule is binding — those are
  the index's and the compile's to say. The same authority boundary ADR-028 draws for
  memory holds here.
- **Off by default, and then still honest.** `provider = none` makes no call and
  changes nothing. With a provider, every decision is logged to the call log with its
  site, provider, confidence and latency, and every envelope that used one says so.
  A decision in the unsure band never changes served content.
- **No new tool joins a surface** (product-vision standing non-goal): the port is a
  daemon-side seam and one loopback endpoint the hook already had budget for.
- **Egress is opt-in and named.** I14 is amended, not broken: the decision port is
  the second sanctioned off-host egress after compilation, armed only by
  operator-provided config, never from a hook process, and disclosed.

## 4. Measurement before default-on (ADR-041: earn the place)

Each site has a free or cheap A/B on the same tree with `provider = none` against
`provider = llm`: the compile (sections compiled, served rules lost, dollars, wall
clock; bar: no served rule lost, under ten minutes and a dollar), obligations
(gold-source rate of `obligations` on guards-fathom and a rebuilt findings corpus),
routing (fraction of issue-shaped prompts reaching `localize`, hit@1 on LCA). The
bench needs no driver change: a corpus variant whose arm setup appends the `[decide]`
block is a different corpus hash, hence a different cell key.

## 5. Considered and rejected

- A new MCP tool ("decide"). Rejected: no tool joins a surface without call-log
  justification, and the decisions are internal to serving and compiling.
- Calling the provider from the hook. Rejected: I14, and a hook must fail open in
  milliseconds; the daemon owns the deadline and the log.
- Letting a model re-rank across scope tiers. Rejected: ADR-035's scope law is the
  one part of serving an adopter can predict by reading the rule's path; a model may
  order within it, not over it.
- Recording every decision as a claim. Rejected: thousands per session of ephemeral
  ranking are instrument truth (the call log), not durable evidence; the
  evidence-producer slot (kind `model`, cap `sme-stated`) is reserved for a future
  site whose decisions persist (findings triage).
