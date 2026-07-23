# Wave 4 — Pack taxonomy + the adoption kit

Two coupled deliverables (owner directive 2026-07-23): (1) the four-stratum
pack taxonomy recorded durably and implemented as the layer ecosystem's
organizing vocabulary; (2) adoption made excellent — skills for Claude Code
and Codex, AGENTS.md/CLAUDE.md/hooks wiring guidance, and a coherence pass
over the whole documentation surface.

Design (normative):
- **Taxonomy (docs/design/pack-taxonomy.md + ADR-024).** Four strata, each
  with its own authorship and trust model:
  | stratum | fathom realization | authored by | trusted via |
  |---|---|---|---|
  | dialect | extractors + tool identity (code, not packs) | fathom | golden tests, deterministic |
  | provider | DOES NOT EXIST — parked pack type | fathom or vendor | eval-gated, staleness vs upstream |
  | platform | .fathom/ project packs (obligations, rules, bindings) | the org | compiled from org corpus; on-disk eval evidence required (D3) |
  | concern | layer catalog + rule packs + analyzers | fathom + org extensions | eval-gated embedded; entry-conditioned growth |
  Key recorded insights: the premade-vs-custom axis IS the
  embedded-vs-.fathom/ union with the D3 trust seam; provider packs are the
  first pack type whose corpus drifts with an UPSTREAM, so watermark/
  staleness machinery applies to pack corpora; the Platform stratum
  formalizes ADR-011 (org content is configuration, never product).
  Internal org names from any source never enter this repo.
- **Stratum as vocabulary, not machinery.** `#Layer` gains an optional
  `stratum: "concern" | "platform" | *"concern"` field (additive; embedded
  packs annotated; project packs default sensibly); `fathom layers` and
  docs/layers.md display it. No new subsystems.
- **Parked with triggers (recorded here, not built):** provider pack type
  {trigger: a named consumer needing an external technology's resource
  semantics explained}; practice/concern pack library growth {trigger per
  pack: real consumer + corpus, eval fixtures, kill criterion — the
  playbook pack's ≥15-findings condition is the model}.
- **Skills (the adoption kit's core).** Shipped, versioned skill sources
  under `dist/agents/skills/`, installable via `fathom init --skills`
  (copies into the target repo's .claude/skills/ for Claude Code; renders
  the Codex prompt-file variant into ~/.codex/prompts/ or the repo per
  Codex conventions — honest capability table where Codex differs). Minimum
  set, each a SKILL.md with frontmatter per Claude Code's format:
  `fathom-plan` (scaffold a gate-ready grammar-v2 plan — generalize this
  repo's existing local plan skill), `fathom-verify` (run + interpret the
  gate, incl. plans status/next), `fathom-onboard` (wire a repo end to end:
  ingest, MCP registration, hooks, .fathom/, verification checklist).
  Skills must only reference shipped verbs; every command verified.
- **AGENTS.md + CLAUDE.md wiring guidance.** `fathom init` scaffolds a
  marker-delimited fathom section into CLAUDE.md AND AGENTS.md
  (create-or-append, idempotent, never clobbers user content); the section
  content is the index-first workflow + hook expectations, kept short with
  pointers to docs/adoption/. Adoption docs gain the AGENTS.md story and
  the skills install flow for both harnesses.
- **Coherence pass.** README routes newcomers to adoption/ in one hop;
  every adoption doc's Maintained line current; catalog cross-refs
  resolve; the taxonomy vocabulary used consistently across layers docs.
- **The flywheel (owner directive: build the info-gathering loop in).**
  Productize the dogfood discipline that has driven this program — observe
  → root-cause → fix → close — so every adopting repo runs it, ALL
  LOCAL-ONLY (no hosted anything, risk register R1; nothing phones home):
  (1) `fathom instrument --adoption`: one deterministic-given-inputs report
  from the call log + store — per-surface traffic, top tools, zero-call
  list, claims-attach hit rate, verify-gate catch count, open observation
  count — the flywheel's gauge, shipped as a CLI report, not a dashboard;
  (2) `fathom observe`: structured friction capture (symptom, tool,
  expected/actual, reproduction) appended to the repo's
  `.fathom/observations.md` in the dogfood-observations entry shape, with
  open-count surfaced by the instrument report; (3) the `fathom-feedback`
  skill teaches AGENTS to file observations the moment they hit friction —
  agents are the primary users, so agent-filed observations are the
  flywheel's intake. The loop closes exactly as this repo's does:
  observations graduate into fixes and CHANGELOG entries or die with a
  recorded reason.

## Tasks

- [x] docs/design/pack-taxonomy.md + ADR-024 recording strata, trust models, the two parked pack types with triggers {id: TX-1, symbol: LoadLayerIn, package: internal/semantic, skip_test: true, skip: "design doc + ADR anchored to the union loader the taxonomy organizes; TX-2 carries the code coverage"}
- [x] #Layer stratum field (schema + embedded mirror + drift test), embedded packs annotated, layers CLI + docs/layers.md display it {id: TX-2, symbol: LoadLayerIn, package: internal/semantic}
- [x] dist/agents/skills: fathom-plan, fathom-verify, fathom-onboard skills (Claude Code SKILL.md format + Codex prompt variant from one source); every referenced command verified against shipped code {id: AK-1, symbol: cmdInit, package: cmd/fathom, skip_test: true, skip: "skill sources are data verified by AK-2's install tests + the critic's command-accuracy pass"}
- [x] fathom init --skills installs skills for the detected/selected harness; idempotent; tested against scratch target repos {id: AK-2, symbol: cmdInit, package: cmd/fathom}
- [x] fathom init scaffolds marker-delimited fathom sections into CLAUDE.md and AGENTS.md (create-or-append, idempotent, user content never clobbered); tested incl. re-run and pre-existing-content cases {id: AK-3, symbol: cmdInit, package: cmd/fathom}
- [ ] Adoption docs updated: skills install + AGENTS.md story in claude-code.md and codex.md; SDD guide links fathom-plan; README one-hop routing; Maintained lines current {id: AK-4, symbol: cmdInit, package: cmd/fathom, skip_test: true, skip: "docs rows anchored to the init command that wires what they document; accuracy enforced by the critic pass"}
- [ ] Coherence sweep: taxonomy vocabulary consistent across layers docs; cross-refs resolve; catalog stratum column present {id: AK-5, symbol: cmdLayersList, package: cmd/fathom, skip_test: true, skip: "docs sweep anchored to the CLI that renders the catalog; TX-2's tests cover the code"}
- [x] fathom instrument --adoption: per-surface traffic, top/zero tools, claims-attach rate, gate catches, open observations — deterministic given inputs, golden-tested {id: FW-1, symbol: cmdInstrument, package: cmd/fathom}
- [x] fathom observe: structured friction capture into .fathom/observations.md (entry shape validated, append-only, idempotent re-runs safe); tested {id: FW-2, symbol: cmdObserve, package: cmd/fathom}
- [x] fathom-feedback skill: agents file observations on friction, referencing FW-2's verb; ships with the AK-1 skill set {id: FW-3, symbol: cmdInit, package: cmd/fathom, skip_test: true, skip: "skill source verified by the install tests + critic command-accuracy pass, same as AK-1"}

## Deferred

- [-] Provider pack type (schema, corpus watermarking vs upstream, first pack) {id: TX-3, disposition: deferred, reason: "no consumer has named a provider whose semantics fathom must explain; building ahead of demand is the kit's documented failure mode", trigger: "a named consumer + one concrete provider whose resource semantics an indexed service actually needs"}
- [-] Practice-pack library growth beyond the shipped catalog {id: TX-4, disposition: deferred, reason: "adr-rules/spec-bindings sat eval-perfect with zero production claims for weeks — packs built ahead of activation are theater", trigger: "per pack: a real consumer with a real corpus + eval fixtures + a kill criterion (playbook pack's ≥15-findings condition is the model)"}
