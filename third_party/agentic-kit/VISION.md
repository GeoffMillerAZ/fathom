# VISION

**agentic-kit is the opinionated, deterministic foundation for governed agentic
systems** — the library Geoff's future agentic workloads and harnesses are built
on, so that engineering excellence is inherited, not re-derived, and the known
anti-patterns of agentic systems are structurally hard to commit.

This file states what the kit believes, what it protects you from, what it
refuses to be, and how it stays current. If a change to the kit contradicts this
file, either the change is wrong or this file must be amended first —
deliberately, in its own commit.

## The one-sentence test

> Could a competent engineer start a brand-new agentic workload tomorrow,
> import this kit, and be **compliant with current agentic best practice by
> default** — with determinism, evidence discipline, cost governance, and
> observability inherited rather than rebuilt?

Every addition, refactor, and doc is judged against that sentence.

## Core beliefs (the opinions baked in)

1. **Agentic engineering is data engineering.** Every consequential artifact an
   agent consumes or produces — events, evidence, findings, configs, tool
   schemas, transcripts — is data with a contract. CUE owns the *shape*; Go owns
   the *behavior*. Uncontracted shapes drift, and drifting shapes are how
   agentic systems rot silently. (This is why CUE, not YAML-and-hope: see
   [docs/why-cue.md](docs/why-cue.md).)

2. **Determinism is the default; nondeterminism is injected.** Clocks,
   randomness, network, subprocesses, and model calls live behind seams
   (`now func() time.Time`, injected rand, `LLMClient`, `Embedder`). Anything
   that ran once must be replayable: cassettes for model calls, hash-chained
   event stores and append-only journals for state, golden tests for outputs
   (the hash chain lives in the event store; journals delegate tamper-evidence
   to it). A harness you
   cannot replay is a harness you cannot debug, regress, or trust.

3. **Evidence before assertion; summaries are projections, never replacements.**
   Claims carry grades, spans, sources, and provenance — any output must trace
   back to what justified it. Context compaction and progressive summarization
   are *projections over* ground truth (which stays in journals, event stores,
   and evidence spans), never substitutes for it. Severing the evidence link to
   save tokens is the canonical agentic anti-pattern this kit exists to prevent.

4. **Fail closed, spend bounded.** Gates err toward refusal. Every paid call is
   budgeted before it is made. Every cache, queue, seen-set, buffer, and loop
   has an explicit bound and an explicit behavior at the bound. Cost and blast
   radius are *governed properties*, not hopes.

5. **Escalation is structured, never improvised.** Cheap-to-capable model
   escalation happens on declared signals (the ladder). Refinement loops are
   bounded and escalate to a human through a typed channel
   (operator-elicitation), not by giving up silently or looping forever.
   Stop causes are values, not exceptions.

6. **Honest contracts only.** No capability theater: the wrapper is
   defense-in-depth, not a security boundary; bounded-exec is resource
   confinement, not a sandbox; DLP is best-effort shape redaction; append-only
   stores have no deletion story. Overclaiming is treated as a defect class
   equal to a failing test — it is how trust in a foundation dies.

7. **Stdlib-only durability.** Zero Go dependencies means zero dependency
   conflicts in any consumer, forever, and WASM portability for free. The kit
   must be safe to embed in *everything*, which means it may depend on
   *nothing*.

8. **The gate is the contract.** `task validate` (gofmt, vet, `cue vet`
   including fixtures, `go test -race` across every package) plus a green
   downstream consumer (`task fathom-check`) is what "done" means. TRACKING.md
   records what is built and tested; CHANGELOG.md records what broke and why;
   tags are cut only at green gates.

## What the kit protects you from

The concrete anti-pattern → opinionated-tool mapping lives in
[docs/practices.md](docs/practices.md). Headlines: lossy summarization without
evidence retention, unsourced claims, improvised escalation, unbounded
loops/spend, trusting tool and model output, live-API-only testing, invisible
runs, unaudited prompt drift, tool definition sprawl, schema/doc drift, silent
truncation. Each entry names the kit packages that enforce the practice and is
honest about what is shipped versus partial versus roadmap.

## What the kit is NOT

- **Not a framework.** It never owns `main()`, your loop, or your process
  model. Kits compose foundations; you compose kits. Everything is importable
  piecemeal.
- **Not provider lock-in.** `LLMClient`/`Embedder` are the contracts; Anthropic
  is the first battery, not the architecture.
- **Not a research playground.** Capabilities land when a real workload pulls
  for them. Speculative abstractions are junk even when they sound visionary;
  deferred ideas are recorded in TRACKING.md with rationale so they are
  sequenced, not lost.
- **Not a compliance checkbox.** The practices are enforced by *running code
  with tests*, not by documentation that asserts compliance.

## How it stays current (practice-watch)

Best practice for agentic systems moves fast. The kit keeps up by process, not
by hope:

1. **Absorb, don't annotate.** When the field's best practice moves (a protocol
   revision, a new failure mode, a new cost lever), the kit absorbs it as an
   opinionated primitive plus a practices.md entry — not as a footnote telling
   consumers to handle it themselves.
2. **Periodic adversarial audit.** The cycle that produced v0.2.0 — full-repo
   survey → adversarial plan review → scoped implementation waves with the gate
   green between waves → multi-lens adversarial code review → fix wave → tag —
   is the recurring maintenance discipline, not a one-off. Every audit hunts
   specifically for: overclaims, unbounded anything, evidence-link breaks, and
   drift between docs and code.
3. **Downstream is the oracle.** Real consumers (fathom today, every future
   workload tomorrow) drive extraction: when a project hand-builds something
   twice, the kit owns it. `task fathom-check` keeps the promise mechanical.

## The junk gate (definition of done for any addition)

A change may land only with: a contract (`schema.cue` where the shape is data),
deterministic `-race`-clean tests that pin its central claim, docs that state
its non-guarantees as plainly as its guarantees, either a consumer or a named
future workload that pulls for it, and the full gate green. "Looks done" is the
enemy; the most dangerous artifact in a foundation is a half-baked capability
that demos well.
