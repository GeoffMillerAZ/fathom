# Fathom — product vision, use cases, and roadmap

*Status: adopted 2026-07-07. Companion to `semantic-layers.md` (ADR-015/016).
Produced by a 3-strategist / 2-judge adversarial panel over the owner's thesis;
verified facts below were checked against this repo, its telemetry, and external
category history during that review.*

> **Engine-assumption note (2026-07-23, ADR-025).** SS-8 deleted the DuckDB
> backend outright once the fleet finished migrating to pure-Go SQLite
> (ADR-020) — the store engine is SQLite-only now, with no DuckDB compiled in
> anywhere. This doc's entry-conditioned H3/vector mentions below (`.duckdb`
> spec, `ATTACH`-based federation, DuckDB VSS) were written when DuckDB was
> still the store engine and describe mechanisms that assumed it; none of
> them have fired their entry conditions yet, so nothing here needed to
> change on short notice, but whichever one fires first needs a technical
> re-check against the SQLite-only reality before being executed — not a
> silent port of the DuckDB-specific mechanism.

## 1. What fathom is, can be, and should be

**Is:** a code-intelligence + governed-evidence engine — deterministic index,
evidence tiers, staleness honesty, path-scoped MCP surfaces, call telemetry.

**Can be:** the **governed knowledge engine** — the store format and reference
engine for *expert modules*: portable, self-describing knowledge artifacts
(deterministic facts + LLM-compiled claims with citations, tiers, freshness) over
any expert corpus, standalone or federated.

**Should be (the discipline):** one product that earns each ring before building
the next. The category precedents are unambiguous about what endures: SQLite's
*file format* outlived every app; Terraform's moat is the provider protocol;
dbt's is the project DSL. Fathom's durable asset is therefore **the governed-claim
store format** — a versioned, self-describing `.duckdb` spec third parties can
target — with the binary as reference implementation. Becoming a platform is an
*event* (a third party targets the format), not a roadmap phase.

## 2. Product shape: one engine, one binary, multiple surfaces

**Not multiple products.** Per-use-case adaptation is already solved structurally:
path-scoped tool surfaces (ADR-014) + per-use-case semantic layers = *editions
without forks*. Two amendments to the original thesis, from the panel:

1. **The store format is the asset.** D-EM2 ("the store is the module") is
   currently prose, not code — observations anchor by absolute machine-local
   paths, so a store cannot leave its build machine. Portability becomes a *birth
   property* of the claims schema (H1), then a published format spec (H2).
2. **Delivery is configuration, not agent habit.** "A tool the agent may choose to
   call" is a zero-moat position — the harness owns default policy, and our own
   telemetry proves the habit gap (fathom-vs-grep share measured 0–7% in the
   fleet). The durable channels are **forced by configuration**: SessionStart /
   PostToolUse hooks that *inject* payloads, and **blocking verify gates** in
   pre-push/CI. The MCP tool surface remains the secondary pull path.

One added discipline: **the tool surface shrinks before any external audience
sees it.** A solo maintainer publishing 86 tool contracts signs 86 support
promises; a retroactive 30-day generalize-or-kill pass over the call log prunes
the surface before release.

## 3. Use cases — where fathom can be world-class (and where not)

| Use case | Verdict | Excellence bar |
|---|---|---|
| **Evidence-backed verification** ("done means proven": verify_plan as a blocking gate) | **World-class — the headline.** Already shipped, deterministic, $0 marginal, immune to agent habit, no shipped competitor. | ≥90% precision catching false-"done" items at ≤10% false-flag rate on a labeled corpus, running as a blocking gate in ≥2 repos for a month |
| **Agentic coding companion** (context hot loop + injected obligations) | **World-class — with delivery flipped to forced channels** and cross-file call-edge depth in adopters' languages. Harness vendors bundle per-repo, cloud, embeddings-flavored context — not a fleet-wide governed store with tiers and per-claim staleness. **2026-07-24 update (docs/plans/home-bench.md H3-1):** the index-native/home-turf bench also ran and did NOT confirm a decisive win — equal success 12/13 vs 12/13; the surviving honest pitch is equal answers, cheaper on 8/13 tasks (outlier-driven aggregate premium), 2–3× faster wall time, with governance (verify gate/obligations/staleness) itself unmeasured by that bench. **SUPERSEDED 2026-07-24 (second swing, docs/plans/trust-and-tokenomics.md T5-8, win-audited): after the causal fixes (read_source, line anchors, O12 trio) the same unchanged criteria PASSED — 13/13 vs 11/13 success at 0.570× tokens on home turf; the lookup bench improved to 1.96× (retraction of ×10 still stands, <5×). Honest pitch: strictly better answers at ~2× fewer tokens, 2–3× faster.** | ≥90% of edits in indexed repos receive an injected payload; ≥40% fathom-share of discovery actions sustained 4 weeks; context-bench win on success + cost — the "≤1/10th tokens" form is retracted after the first run measured equal success at 1.38× aggregate tokens / 2.81× wall time (docs/plans/falsifiability.md B1-3) |
| **Packaged expert modules** (engineering-adjacent first: runbooks, standards, findings) | **World-class as the north star** — gated on three things currently false in code: portable anchors, reinterpretable tiers, non-git freshness. All three are cheap at schema time (H1) and expensive forever after. | A store compiled on machine A serves intact citations + honest staleness on machine B, zero recompile; then one external party runs a module without us in the loop |
| **Incident investigation with precedent** (failure-mode playbook) | World-class *later* — entry-conditioned: the corpus is ~5 resolved observations today; compiling on it would fail our own precision gate. | Entry: ≥15–20 resolved findings; then ≥70% of a 20-incident replay set surfaces the true root-cause class in top-3 checks |
| Intent-aware change review | Good, not differentiated — until the invariant layer earns its revival evidence. Position as the evidence feed reviewers cite, not a competing reviewer. | If revived: ≥5 reviewer-accepted invariant findings per 100 PRs that no co-installed tool flagged |
| Org knowledge-health / standards-drift observability | Defer — real budget line, but an org product needs an org buyer. Everything it needs accrues free from claims + arch_conformance. | When revived: 100% of failing scorecard cells link to a code-cited, tiered claim |
| Governed onboarding / comprehension | Good, not differentiated — no decision moment, no attached consumer (the llms.txt failure). Free byproduct of brief/repo_map; staffed never. | Zero dedicated effort; first-hour grep count in a new-repo session <10 |
| Regulated-domain modules via domain connectors (PDF/Confluence/portals) | **Avoid.** No dogfood loop, liability-shaped error costs vs a 15–20% citation-hallucination base rate, and it turns fathom into a connector company (the unwinnable Glean game). | Reconsider only if an external party ships a conforming module on the published spec with zero connectors from us |

**Standing non-goals** (add to the permanent list): no domain connectors; no
web-scale code search; no IDE language services; no embeddings-first chat; no
tool joins a surface without call-log justification; every layer ships with its
consumer, eval fixtures, and a kill gate; costs scale with change size, never
repo size.

## 4. Roadmap — prove → prune → prepare; publish → deepen; ecosystem on evidence

### H1 — PROVE, PRUNE, PREPARE (internal; the wedge must be true at home first)

1. **Week 1 — repair the instrument.** `fathom-adoption.sh` cannot currently see
   the dogfood repo (post-rename name mismatch vs transcript dirs); verify
   `--call-log` survives restarts; commit a weekly adoption baseline. No gate
   clocks and no adoption claims until the instrument has two clean weeks.
2. **Verify becomes the lead.** Blocking `verify_plan` lefthook pre-push / CI gate
   in ≥2 fleet repos this month; record the demo catching an `[x]`-but-unproven
   item. "Context + proof" is the launch story no competitor has.
3. **Portability tax, paid before the claims DDL lands** (ADR-017 to write):
   repo-relative anchors + store-level root remap + failing
   `TestStorePortability_RoundTrip`; Connector SPI gains `Watermark()` (git HEAD
   for Local, manifest hash for Faux — freshness goes domain-neutral); claims
   carry a `tier_profile` reference column (data now, machinery at its revival
   evidence). Accepted cost: the obligations bullet slips 1–2 weeks.
4. **Obligations tracer bullet, hook-first** (amends `semantic-layers.md` §4.5/4.6):
   delivery plan A is a PostToolUse(Edit|Write) hook *injecting* the ≤150-token
   payload; the companion tool is secondary. Gate metric rewritten: **injection
   coverage** of edits in indexed repos + the four default-violation classes
   trending to zero — not call-rate, which measures a habit already falsified.
   `claim_events` table lands with the DDL (append-only audit is cheap now,
   unrecoverable later); the `claim_feedback` tool waits for the gate.
5. **Surface pruning + miss taxonomy.** Retroactive 30-day generalize-or-kill over
   the 86 tools; classify every transcript grep that bypassed fathom and fix the
   top three bypass classes — diagnosis over more push.
6. **Distribution prep (not release):** tag agentic-kit v0.1.0 + delete the
   `go.work`/`replace` coupling; `fathom init` (one command: ignore file, per-user
   default store, MCP registration, operating contract) + `fathom daemon install`;
   CI release matrix; **context-bench built and run privately** (~50 discovery +
   verification tasks over ~10 OSS repos vs grep/Read, LSP-over-MCP, repomap).

**H1 exit criteria = the release gate** (ship the day they're green, whether month
3 or 6): instrument valid ≥4 weeks; injection coverage ≥90%; verify gate live in
≥2 repos ≥4 weeks; surface pruned; portability round-trip passing; `fathom init`
works on a cold clone; context-bench shows a defensible win. If these can't go
green internally, do not release — re-examine the wedge.

### H2 — PUBLISH AND DEEPEN (the release is the boundary event)

- Repos public, tagged binaries, brew tap, MCP registry listings, Claude Code
  plugin bundle (MCP + hooks + `/verify` command — adoption as one install);
  context-bench published (the falsifiable "best" claim).
- **Depth over breadth in extraction:** cross-file caller resolution for TS/JS,
  Python, Java (then Ruby/Rust) with goldens — `find_callers`/`blast_radius`/
  `test_impact` must work for an external TS adopter as they do for Go. The
  30-language breadth program becomes demand-driven (this re-scopes the currently
  active language-extraction priority).
- **Store/claim-format spec v0** (replaces `fathom pack` as the deliverable): docs
  + CUE schema + explicit stability guarantees; publishable only because the
  portability test already passes. The spec names the category — *governed
  knowledge engine* — and requires conforming modules to carry corpus selectors,
  watermarks, and recompile rights (the freshness-doctrine answer to detached
  knowledge). The pack CLI, when it comes, is a thin serializer. **2026-07-28
  update:** the v0 spec landed (`docs/spec/store-format-v0.md`,
  `docs/spec/expert-module-v0.md`) — see `docs/vision/
  12-governed-knowledge-engine.md` for what it unlocks next and the owner
  decision to pull the module-host/chat capability forward from H3 ahead of
  this section's external-adoption entry condition, which remains unmet.
- **Layer 2 = failure-mode playbook**, entry-conditioned (obligations gate passed
  AND corpus ≥15–20 findings), compiled from a non-git folder corpus so
  watermarks/portability/tier-profiles get a real consumer. The generic layer
  runtime is extracted only now — generalize from two instances, not one.
- **User #2 probe, bounded:** one-page "auditable AI evidence" brief + one demo
  store for the compliance-engineering engagement. One brief, one store — not a
  roadmap track.

### H3 — ECOSYSTEM, strictly entry-conditioned (evidence, not dates)

- **Federation** = DuckDB `ATTACH` over N conforming portable store files behind
  the existing `pkg/platform` machinery (one module = one store; the engine never
  goes multi-tenant). Entry: ≥2 real external module consumers, or ≥100 external
  installs + ≥60% internal fathom-share sustained 4 weeks.
- Compliance/audit edition (air-gapped, audit-trailed): entry — an active named
  regulated engagement. Org health dashboards: entry — an external org buyer.
- Multi-expert systems ("chat with expert modules", cross-module preprocessing):
  orchestration composes *above* the engine over federated stores; fathom's job
  is to remain the best substrate underneath.
- Vectors stay evidence-gated exactly as designed (DuckDB VSS, additive in-store
  column, never a second store).

## 5. The one-sentence strategy

Prove the wedge at home with forced-channel delivery and a repaired instrument;
lead externally with *proof* (evidence-backed verification), not context; make
every claim born portable so the store format — not the binary — becomes the
category; and let each ring's existence be earned by telemetry, eval gates, and
entry conditions rather than ambition.
