# Repo strategy — what belongs in fathom, what earns its own repo

**Status:** handoff doc, owner-commissioned 2026-07-30 ("if I want a well
architected review tool and code review tool should those be separate repos?
… if they would work as a monorepo now, i'd be down with that and split it
out later"). Written to be re-read months from now when the same question
comes up about something else.

## 1. The decision rule (one test)

> **Could this ship a release on a different day than fathom, and would
> anyone notice?**

If no, it is a package, not a repo. Everything else is elaboration:

| Signal | Same repo | Own repo |
|---|---|---|
| Coupling | reads fathom's store **and** writes it | consumes a stable published contract only |
| Contract | still moving (fields added weekly) | frozen enough to version |
| Consumers | only fathom | someone outside the fleet, or a second in-fleet consumer |
| Release | must land together to be correct | genuinely independent cadence |
| Boundary proof | needs a lint to stay honest | the repo boundary *is* the proof |

The cost of getting it wrong is asymmetric. A wrong monorepo costs you a
directory move later. A wrong split costs you cross-repo PRs, version pins,
and a contract you must freeze before you know its shape — every week, until
you undo it. **When genuinely torn, monorepo now; split on a trigger.**

## 2. Verdicts for the review work

**Review findings storage — fathom, and never separate.** A finding is a
governed claim: `kind:"finding"`, line-hashed citation, tier, supersede
lineage, `code_drifted`. It lives in the same tables as obligations and ADR
rules because its whole value comes from joining to symbols, call edges, and
scopes that live there too. Splitting the storage severs exactly the join
that makes findings worth keeping — you would be left with an issue tracker.
This is not a close call.

**The reviewer engine (`review_diff` and successors) — fathom.** It is
coupled at *both* ends: it reads the index (blast radius, obligations,
conformance, spec coverage) and writes claims back. A thing coupled at both
ends of the same store is a package in that store's repo. Note what it would
be without fathom: a generic LLM reviewer with no call graph, no governing
rules, and no memory of what it already said — i.e. the commodity product the
whole program exists not to be.

**Review UI / dashboards — `fathom-studio`, already separate, correctly.**
This one *is* a renderer over a contract, and law 10 (verbs first) makes the
boundary provable: the studio imports no fathom internals and reaches the
engine only through verb JSON and public packages. That repo split works
because the discipline came first — not because splitting is inherently good.

**CI / forge integration (a GitHub App, a CI action) — its own repo, later,
on a trigger.** This is a *delivery surface*: it takes a diff, calls fathom,
posts results. Thin, independently released, plausibly consumed by people who
never run the daemon. Trigger to split it out: the first time a non-fleet
consumer needs it, or the first release where its cadence actually diverges.
Until then it is a subcommand.

## 3. Keeping a future split cheap (do this in-repo, now)

A monorepo only stays reversible if the seams are real. Three rules, all
already precedented in this codebase:

1. **Package boundaries mirror the eventual repo boundaries.** The reviewer
   engine is `internal/analysis/review` + the findings claim bridge; it must
   not reach sideways into `cmd/fathom` or the MCP layer. If a split ever
   happens, it is `git mv` plus a `go.mod`.
2. **Every capability is a verb first (law 10).** Anything a UI needs is
   reachable as `fathom <verb> --json`. This is what made the studio split
   free, and it is what would make any future split free.
3. **Contracts get a schema and a drift test.** The precedent is
   `store-format-v0` (a spec whose conformance test parses the spec's own
   markdown tables) and the `module_query` wire contract (pinned by
   raw-JSON key tests on both sides). A contract with a drift test can be
   published; one without cannot.

## 4. The rule stated as advice

Split when a **boundary already exists and is being enforced**. Do not split
to *create* a boundary — that is how you end up maintaining two repos to
enforce a rule a fifteen-line lint would have enforced for free.

Applied here: build the review work as packages in fathom now, with the seams
above. If a standalone reviewer product is ever wanted, the boundary will
already be there and the move is mechanical.
