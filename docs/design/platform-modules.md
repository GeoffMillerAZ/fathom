# Platform modules in the coding loop — expert knowledge serving the agentic coding aid

**Status:** adopted design, owner-commissioned 2026-07-29 ("is it possible and
even desirable to allow the knowledge expert part of the product to enhance
the agentic coding aid part?"). Extends `docs/vision/13-expert-suite.md`
(new program phase O, registered in its §9 table by amendment) and
ADR-024's platform stratum. Normative for the phase-O plan.

## 1. The finding: the taxonomy promised this and nothing delivers it

ADR-024 defines the **platform stratum**: an org's own standards, guides,
and platform docs, trusted via org-corpus compilation + on-disk eval
evidence. Its only realization today is `.fathom/` packs compiled
**per-repo**: N repos ⇒ N copies of the same org knowledge, N compiles, N
independent drift clocks, zero shared freshness story. Meanwhile expert
modules (compile once, portable, watermarked, conformance-checked) are
consumable only via chat and `/rpc/modules` — never by the channels the
benches proved (edit-hook injection, obligations serving, review).

**Decision: an expert module is the delivery vehicle for the platform
stratum.** Org-wide knowledge compiles ONCE into a module; consumer repos
*bind* it; the coding loop serves its rules through the existing forced
channels with module-tagged provenance. Repo-local concern layers
(obligations/adr-rules/spec-bindings about THIS repo) are untouched — this
is not a replacement, it is the distribution half the stratum was missing.

## 2. Non-redundancy, stated precisely

| | Concern layers (today) | `.fathom/` platform packs (today) | Platform modules (this design) |
|---|---|---|---|
| Corpus | this repo | this repo's copy of org docs | the org's one canonical corpus |
| Compile | per repo | per repo | once, centrally |
| Freshness | per repo watermark | N independent, silently divergent | one watermark, one staleness verdict everywhere |
| Serving | edit hook + tools | edit hook + tools | same channels, module-tagged |
| Consumer | this repo only | this repo only | every bound repo + chat + MCP |

`.fathom/` project packs remain correct for genuinely repo-specific rule
authoring. Copying org-wide content into them is now an anti-pattern the
docs should steer away from once phase O ships.

## 3. Design (normative)

### 3.1 Binding is consumer-side, explicit, and scoped

A repo opts in via `.fathom/modules.cue` (schema: `schema/binding/`),
validated by the same CUE discipline as every config surface:

```cue
bindings: [{
    module:  "platform-standards"        // mounted module name (daemon --module)
    scopes?: ["infra/**", "**/*.go"]     // optional consumer-side restriction;
                                         // absent ⇒ the module's own scope globs apply as written
    max_rules?: 2                        // per-edit budget contribution cap (default 2)
    min_tier?:  "doc-claimed"            // serve nothing below this tier
}]
```

Doctrine: **no org rule ever reaches a repo that did not bind it.** Central
mounting (the daemon) makes a module *available*; only the repo's own
committed binding makes it *apply*. The binding file is reviewable,
diffable, and lands through the repo's normal review gates.

### 3.2 Scope semantics: module authors write consumer-shaped globs

A platform module's claims carry scope globs written against **consumer
trees** (`**/*.tf`, `**/Dockerfile`, `cmd/**`) — the existing scope grammar
unchanged. The optional binding `scopes` intersects (never widens) the
module's own scopes. No path remapping in v1; a remap field may be added
by evidence, not speculation.

### 3.3 Serving: same channels, federated, budget-honest

- **Edit hook / `obligations` tool:** the daemon's obligations lookup for a
  file consults, in addition to the repo's own claims, every module the
  repo binds — via the existing `modulehost.FederatedObligations` machinery
  (phase G), filtered by binding scopes/tier, capped by `max_rules`.
  Module rules render **module-tagged** (`[platform-standards]`) with tier
  and staleness exactly like the federation surface serves them; the
  repo's own rules keep priority within `MaxObligations`; displaced counts
  (O14) report repo-rules and module-rules displacement separately —
  never a silent eviction in either direction.
- **`module_query` / `pack_context` / `brief`:** bound modules are
  eligible evidence sources for concern queries in a coding session,
  provenance-tagged, tier-ranked below same-tier repo-local evidence on
  ties (the repo knows itself best; the platform informs).
- **Review (`review_diff`):** phase-O v1 serves module rules into the
  obligations dimension it already composes. A dedicated
  standards-conformance dimension is deferred with a named trigger: first
  documented case of a module rule that review served but a reviewer
  needed machine-checked (not just cited) enforcement for.
- **Staleness honesty end-to-end:** a bound module whose staleness verdict
  is not fresh serves its rules **flagged** (same never-silent contract as
  repo claims); `unknown-detached` is stated, not hidden.

### 3.4 Trust boundary

Module rules are advisory obligations with provenance — they are **never**
auto-promoted to `verified`, never override repo-local claims, and never
participate in `verify_plan` gating (a plan row is proven by the repo's
own evidence only). The module's eval evidence + readiness level
(phase J's ladder — only `servable`+ modules may be bound; doctor warns on
binding anything lower) is the trust instrument.

### 3.5 Instrumentation (the flywheel applies)

Injection stamps and `research report` gain a module dimension: rules
served per module, displacement, staleness-flagged serves. A module that
serves zero rules across a measurement window in every bound repo is a
kill-clock candidate — law 8's population precondition applies unchanged.

## 4. What this does NOT do (v1 non-goals)

No auto-binding, no org-push of bindings, no path remapping, no module
rules in verify gates, no provider-stratum content (TX-3 stays parked), no
new MCP tools (the `obligations` tool's response gains module-tagged
entries — same tool, richer honesty).

## 5. Phase O deliverables (the gated plan derives its rows from here)

1. `schema/binding/` CUE schema + loader (modspec-style dual mode) +
   `fathom init` scaffold note.
2. Daemon: binding-aware obligations serving (repo claims ∪ bound module
   rules; budget + displacement accounting split; staleness flags).
3. Edit hook rendering with module tags; hook payload stays ≤ the existing
   token budget — module rules count inside it, never on top of it.
4. Doctor checks: binding names an unmounted module (loud), binding names
   a module below `servable` (warn), binding file invalid (loud).
5. Docs: use-case doc (`docs/use-cases/platform-modules.md`), adoption
   guide section, mcp-tools.md `obligations` row update, ADR-024
   amendment pointer.
6. Bench honesty: phase-O ships instrumented (3.5); its adoption claim
   waits for law-8 qualifying traffic like everything else.
