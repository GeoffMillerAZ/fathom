# VISION

**agentic-kit makes reliable, evidence-backed agentic work easier to build,
verify, and improve across workloads and providers.** It turns recurring engineering
lessons into reusable contracts, deterministic controls, composed workflows, and
regression tests. A lesson learned by one workload should improve the next.

This file states what the kit believes, what it protects you from, what it
refuses to be, and how it stays current. If a change to the kit contradicts this
file, either the change is wrong or the intent must be amended deliberately with a
decision record. The target below is an ambition; current coverage and its limits
live in [docs/practices.md](docs/practices.md) and the package contracts.

## The one-sentence test

> Can an engineer choose a workflow and a supported provider or coding harness,
> inherit appropriate safeguards, demonstrate that they work, trace the result
> to its evidence, and contribute a lesson that another workload can reuse?

Every addition is judged against that sentence. The product is the working path from
setup to verified outcome: library compositions, CLI/MCP adapters, native Claude Code
and Codex integrations, and executable examples. Importable foundations serve that path.
Success criteria: [workflow assurance](docs/intent/assurance.md).

## Core beliefs (the opinions baked in)

1. **Agentic engineering is data engineering.** Every consequential artifact an
   agent consumes or produces — events, evidence, findings, configs, tool
   schemas, transcripts — is data with a contract. CUE owns the *shape*; Go owns
   the *behavior*. Uncontracted shapes drift, and drifting shapes are how
   agentic systems rot silently. (This is why CUE, not YAML-and-hope: see
   [docs/why-cue.md](docs/why-cue.md).)

2. **Determinism is the default; nondeterminism is injected.** Clocks,
   randomness, network, subprocesses, and model calls live behind seams
    (`now func() time.Time`, injected rand, `LLMClient`, `Embedder`). Record the
    inputs needed to replay decisions: cassettes for model calls, hash-chained
   event stores and append-only journals for state, golden tests for outputs
   (the hash chain lives in the event store; journals delegate tamper-evidence
   to it). A harness you
    cannot inspect or replay is difficult to debug and regress. Replay does not
    reproduce a live model's internal reasoning or authorize repeating an external effect.

3. **Evidence before assertion; summaries are projections, never replacements.**
    Factual prose maps each claim to specific, versioned source evidence; inferences
    retain their derivation and uncertainty. Valid references, semantic support, and
    source truth are separate properties. Context compaction and progressive summarization
   are *projections over* ground truth (which stays in journals, event stores,
   and evidence spans), never substitutes for it. Severing the evidence link to
   save tokens is the canonical agentic anti-pattern this kit exists to prevent.

4. **Explicit authority, bounded work.** Required controls refuse before the effect
    when they cannot establish permission. Every paid attempt, including retries and
    delegated work, belongs to a budget; estimates and measured spend stay distinct.
    Request-path structures and loops have bounds and behavior at the bound. Durable
    evidence stores have an explicit retention policy; retaining evidence is not a
    license for unlimited sensitive-data accumulation.

5. **Selection is evidence-based; escalation is structured.** Workload-specific
   choices across agentic engineering use applicable criteria and versioned outcome
   evidence, not a universal excellence score. Model, retrieval, architecture and
   red-team assessments share an evidence lifecycle, not one rubric; hard requirements
   precede soft optimization. Models must be eligible deployments. Capability escalation happens on
   declared signals (the ladder); missing evidence, outages and missing authorization
   need different responses. Refinement loops are
   bounded and escalate to a human through a typed channel
   (operator-elicitation), not by giving up silently or looping forever.
   Stop causes are values, not exceptions.

6. **Honest contracts only.** No capability theater: the wrapper is
   defense-in-depth, not a security boundary; bounded-exec is resource
   confinement, not a sandbox; DLP is best-effort shape redaction; append-only
   stores have no deletion story. Overclaiming is treated as a defect class
   equal to a failing test — it is how trust in a foundation dies.

7. **A durable, portable core.** The root Go module has no external dependencies.
    Dependency-bearing format handlers remain nested modules. Keep provider and host
    specifics behind explicit adapters; verify portability rather than inferring it
    from dependency count. Consumers retain control of their process and storage.

8. **The gate is the contract.** `task validate` (gofmt, vet, `cue vet`
   including fixtures, `go test -race` across every package) plus a green
    downstream check for affected consumers is the implementation gate. A workflow's
    claim additionally needs a test of its actual enforcement path, including failure
    and missing prerequisites. TRACKING.md records implementation; assurance records
    integration and adoption. Skipped checks and unmeasured outcomes cannot become passes.

## What the kit protects you from

The concrete anti-pattern → opinionated-tool mapping lives in
[docs/practices.md](docs/practices.md). Headlines: lossy summarization without
evidence retention, unsourced claims, improvised escalation, unbounded
loops/spend, trusting tool and model output, live-API-only testing, invisible
runs, unaudited prompt drift, tool definition sprawl, schema/doc drift, silent
truncation. Each entry names the kit packages that enforce the practice and is
honest about what is shipped versus partial versus roadmap.

## What the kit is NOT

- **Not a mandatory framework.** It never requires ownership of `main()` or your
  process model. The optional agent harness supplies a loop; foundations remain
  importable without it. Kits compose foundations; you compose kits.
- **Not provider lock-in.** `LLMClient`/`Embedder` are the contracts; Anthropic
  is the first battery, not the architecture.
- **Bounded research with an engineering exit.** A named consumer or a sponsored
   incubation can justify work. Incubation needs an executable hypothesis, adversarial
   scenarios, maintenance owner, and dated promote/revise/retire decision; it stays
   experimental until proven. See [decision 0004](docs/intent/decisions/0004-bounded-incubation.md).
- **Not a compliance checkbox.** The practices are enforced by *running code
  with tests*, not by documentation that asserts compliance.

## How it stays current (practice-watch)

Best practice for agentic systems moves fast. The kit keeps up by process, not
by hope:

1. **Turn lessons into executable improvements.** A field pattern or consumer incident
   becomes a scoped failure hypothesis, then a regression case and a remedy in an
   existing workflow where possible. Share versioned, reviewed lessons with provenance
   and applicability; private transcripts and recalled claims do not become authority.
2. **Periodic adversarial audit.** The cycle that produced v0.2.0 — full-repo
   survey → adversarial plan review → scoped implementation waves with the gate
   green between waves → multi-lens adversarial code review → fix wave → tag —
   is the recurring maintenance discipline, not a one-off. Every audit hunts
   specifically for: overclaims, unbounded anything, evidence-link breaks, and
   drift between docs and code.
3. **Downstream measures value.** Consumer adoption, deleted duplicate glue, prevented
   failures, setup time, and cost per accepted outcome decide whether an abstraction
   pays for itself. Package counts and repeated replay of a scripted answer do not.
   Record inspected revisions and skipped consumers with every compatibility result.

## The junk gate (definition of done for any addition)

A change may land only with: a contract (`schema.cue` where the shape is data),
deterministic `-race`-clean tests that pin its central claim, docs that state
its non-guarantees as plainly as its guarantees, either a consumer or an incubation
meeting decision 0004, and the full gate green. A composed entry point must prove its
own wiring and failure behavior. "Looks done" is the
enemy; the most dangerous artifact in a foundation is a half-baked capability
that demos well.
