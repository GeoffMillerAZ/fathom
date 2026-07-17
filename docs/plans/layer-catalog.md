# Layer catalog — close M1, prove M2, ship layer 2

Owner question 2026-07-17: are semantic layers at full potential, and
should fathom ship prepopulated layer configs (coding agents, SDD, …)?
Grounded answer (wf_22fe6355): M1 is done and over-delivered (budgets
enforced, verified tier admitted), but two gaps keep layers from being
"just data," and the vision already specifies the target
(docs/vision/02-shape.md §3: `layers/<name>/{layer.cue, prompt.md,
producers.cue, eval/, KILL.md}`; M2 = `fathom layer install <dir>` +
project-local `.fathom/layers/`, with layer 2 shipping as the proof that a
new layer needs no Go changes; 08-roadmap.md names layer 2 = ADR/
architecture-rules and lists a catalog/registry SERVICE as a standing
non-goal).

The two gaps, verbatim from the audit:
1. **Prompt text is still Go** — `promptForLayer` is a switch on layer
   name; an unknown layer silently falls back to the obligations prompt
   (a correctness bug, not just missing polish).
2. **No install/discovery** — every layer is compiled into the binary via
   the embedded manifest; adding one means editing schema.cue and
   rebuilding.
Plus one serving hardcode: `code_drifted` is gated on `KindFinding`
(serve.go/compile.go), so the next layer that wants drift tracking loses
it silently.

Design (normative):
- **Prompt-as-data without identity rotation.** `#Layer.prompt_file`
  (repo-relative), prompts embedded beside the manifest; generic
  `promptForLayer(LayerDef)` loads bytes and derives
  `PromptVersion = layerName + "/" + TextHash(text)[:8]` exactly as today.
  HARD CONSTRAINT: extraction is byte-identical — obligations stays
  `obligations/c02b1aa1`, pinned by test; no fleet recompile is triggered
  by this refactor. Unknown layer name or missing prompt file is a LOUD
  error — the silent obligations fallback dies.
- **`serving.drift_tracked: bool` in #Layer** (findings: true) replaces
  every `KindFinding` gate for scope-fp writing and served `code_drifted`.
- **Pack shape = the vision's, exactly**: `schema/semantic/layers/<name>/`
  holding `layer.cue`, `prompt.md`, `eval/` (labeled fixtures + expected
  extractions for the precision/recall gate), `KILL.md` (metric, window,
  decision date per the admission rule). `producers.cue` reserved, not
  parsed (M3). Obligations and findings MIGRATE into this shape — the
  shipped catalog and the built-ins are the same mechanism, not two.
- **Loader unions three sources**: embedded packs → project-local
  `.fathom/layers/<name>/` → (nothing else; no registry, no remote).
  Presence = enabled (ADR-018 reconciler doctrine: no enabled/disabled
  state machine). Name collisions: project overrides embedded, loudly
  noted in `fathom layers` output.
- **`fathom layers` verb**: `list` (name, kind, tier, corpus, source
  embedded/project, drift_tracked, compiled-claims count per service when
  a store is reachable); `install <dir>` (validates the pack against the
  schema + prompt_file presence + eval/ presence — warns if eval/ is
  empty — then copies into `.fathom/layers/`); `show <name>` (the full
  resolved definition + prompt provenance). Users never hand-author CUE:
  install copies packs; the shipped packs are authored once, here, under
  the gate.
- **Two new shipped packs, each with eval fixtures that PASS the
  gate** (precision ≥0.90 / recall ≥0.75, the semantic-layers.md §4.6
  discipline, run via the existing eval harness through the claude-cli
  subscription lane — no API keys):
  - `adr-rules` (THE layer-2 proof): kind `adr-rule`, tier `doc-claimed`,
    corpus `ADR.md` + `docs/design/*.md`; extracts architectural
    constraints ("stores are born portable", "state is derived never
    driven") scoped to the code they govern. Fixtures labeled from this
    repo's own ADR corpus.
  - `spec-bindings` (the SDD pack): kind `spec-binding`, tier
    `doc-claimed`, corpus `docs/plans/**` excluding README; extracts
    binding requirement lines (the [ID]-carrying obligations of gated
    plans) scoped to their packages — feeding the same serving channel the
    edit-hook already renders kind-aware. Fixtures from this repo's plans.
  If a pack cannot pass the gate after honest prompt iteration, it ships
  with `KILL.md` recording the failing metrics and stays OUT of the
  embedded set — never ship a layer that fails its own admission bar.
- Serving needs zero new code (ObligationsFor is all-layers; edit-hook is
  kind-aware since FT-2) — assert that with an end-to-end test compiling a
  third layer and serving it, since "no Go per layer" is the claim under
  proof.

## Tasks

- [ ] prompt_file in #Layer + embedded prompts; generic promptForLayer(LayerDef); loud unknown-layer/missing-file errors; obligations identity pinned unchanged (obligations/c02b1aa1) {id: LC-1, symbol: LoadLayer, package: internal/semantic}
- [ ] serving.drift_tracked replaces KindFinding gates in scope-fp writing and code_drifted serving {id: LC-2, symbol: DriftedFindings, package: internal/semantic}
- [ ] Pack loader unions embedded packs with project .fathom/layers/; presence = enabled; project-over-embedded override surfaced {id: LC-3, symbol: LayerNames, package: internal/semantic}
- [ ] Built-ins migrate to pack shape (schema/semantic/layers/{obligations,findings}/) with byte-copy drift tests extended {id: LC-4, symbol: LoadLayer, package: internal/semantic, skip_test: true, skip: "layout migration proven by LC-1's identity pin + the existing TestManifestSchemaMatchesSource family; the deliverable is file layout, not a new symbol"}
- [ ] adr-rules pack with eval fixtures passing the §4.6 gate; results recorded in the pack's eval/RESULTS.md {id: LC-5, symbol: CompileLayer, package: pkg/client, skip_test: true, skip: "pack is data + gated eval runs; the compile path is already test-reached — the deliverable is the pack and its recorded metrics"}
- [ ] spec-bindings pack with eval fixtures passing the gate; results recorded {id: LC-6, symbol: CompileLayer, package: pkg/client, skip_test: true, skip: "same rationale as LC-5 — data + recorded gated metrics"}
- [ ] fathom layers verb: list/show/install with pack validation {id: LC-7, symbol: cmdLayers, package: cmd/fathom}
- [ ] Third-layer end-to-end: compile + serve a non-obligations non-findings layer with ZERO layer-specific Go — the no-Go-per-layer claim under test {id: LC-8, symbol: ObligationsFor, package: internal/semantic}
- [ ] Docs: docs/layers.md catalog reference (each pack: when to use, corpus, cost, gate metrics); mcp-tools.md + use-case Maintained lines updated for the layers verb {id: LC-9, symbol: cmdLayers, package: cmd/fathom, skip_test: true, skip: "docs deliverable anchored to the verb it documents"}

## Deferred

- [-] producers.cue deterministic producers + fathom pack/unpack (M3) {id: LC-10, disposition: deferred, reason: "vision 02-shape sequences M3 after M2 proves the pack shape", trigger: "first pack whose corpus is deterministic analyzer output wanting verified tier (also FT-7's trigger)"}
- [-] Pack registry / catalog service or search UX {id: LC-11, disposition: deferred, reason: "standing non-goal (08-roadmap §non-goals); start with a git repo of packs when H3 conditions fire", trigger: "H3 entry conditions per docs/vision/08"}
- [-] runbook-invariants pack (incident investigation) {id: LC-12, disposition: deferred, reason: "no runbook corpus exists in the fleet to write honest eval fixtures against", trigger: "first consumer with a real runbook corpus"}
