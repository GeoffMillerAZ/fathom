# Future directions

Things that might happen. NOT requirements; nothing here authorizes building anything.
An entry exists so today's design does not wall off a plausible tomorrow when keeping the
door open is free, and so agents stop re-proposing what has been considered.

Confidence: **likely** (a commitment or sponsor statement exists) · **plausible** (fits
the vision, no commitment) · **speculative** (must not influence design; listed only to
stop re-proposal). Horizon: **next** · **later** · **someday**. Only likely and plausible
futures may justify a free option today. The repo's own sequence is
`docs/design/product-vision.md` §4 (H1 → H2 → H3) and `docs/vision/08-roadmap.md`.

## F1. CGO-free binary: the nine CGO tree-sitter languages move to the wazero engine (XT-6b)

- Confidence: likely. Horizon: later.
- Motivation: cross-compiled single-binary releases, no C toolchain at build time.
  Evidence: ADR-025 "Consequence", `Taskfile.yml` header, `docs/vision/11` A5 ("XT-6b not yet fired").
- Do not build: a migration ahead of per-language calibration; a second grammar build chain.
- Preserve: every new language lands on `internal/extractor/tsengine` (query-driven `.scm`),
  never on the CGO bindings; `internal/store` stays pure Go.
- Promote when: a language's wazero recall on the calibration corpus meets the CGO
  incumbent's, for each of the nine, or a release must be cross-compiled.
- Drop if: the wasm32-wasi tree-sitter core stops building or a language cannot reach
  parity on the corpus.

## F2. Public release (H2): public repos, tagged binaries, brew tap, MCP registry, plugin bundle

- Confidence: plausible. Horizon: later.
- Motivation: "the release is the boundary event"; the falsifiable "best" claim is the
  published context-bench. Evidence: `docs/design/product-vision.md` §4 H2.
- Do not build: brew tap, registry listings, marketing, a hosted anything.
- Preserve: `fathom init` works on a cold clone; the in-tree kit mirror (ADR-019) keeps a
  kit-less build possible; no org-specific content (ADR-011).
- Promote when: the H1 exit criteria are green (instrument valid ≥4 weeks; injection ≥90%;
  verify gate live in ≥2 repos ≥4 weeks; surface pruned; portability passing; context-bench win).
- Drop if: the owner decides fathom stays a personal force-multiplier (`docs/vision/07` Q4).

## F3. Federation over portable stores (H3)

- Confidence: plausible. Horizon: someday.
- Motivation: one module = one store; orchestration composes above the engine.
  Evidence: `docs/design/product-vision.md` §4 H3, ADR-016 "federation composes above the engine".
- Do not build: a multi-tenant engine; the DuckDB `ATTACH` mechanism as written (engine gone; needs a SQLite re-check).
- Preserve: born-portable stores (ADR-017); `pkg/platform` as the join point.
- Promote when: ≥2 real external module consumers, or ≥100 external installs plus ≥60%
  internal fathom-share sustained four weeks.
- Drop if: no external module consumer appears after F2.

## F4. Layer 2: the failure-mode playbook (incident precedent)

- Confidence: plausible. Horizon: later.
- Motivation: incident investigation with precedent, compiled from a non-git folder corpus.
  Evidence: `docs/design/product-vision.md` §3–4; `fathom findings status` tracks the entry condition.
- Do not build: the pack, the prompt, or a replay harness before the corpus exists.
- Preserve: findings persist as `kind:"finding"` claims; non-git watermarks (ADR-017).
- Promote when: the obligations gate has passed AND ≥15–20 resolved findings exist.
- Drop if: two replay attempts put the true root-cause class in the top-3 for <70% of a 20-incident set.

## F5. Provider pack type (TX-3)

- Confidence: speculative. Horizon: someday.
- Motivation: explain an external technology's resource semantics; the first corpus that
  drifts against an upstream. Evidence: ADR-024.
- Do not build: the pack type, the upstream watermark kind.
- Preserve: `stratum` is already a `#Layer` field.
- Promote when: a named consumer plus one concrete provider an indexed service needs.
- Drop if: platform modules (ADR-024 amendment) cover the need.

## F6. Concern-pack library growth (TX-4): review packs, compliance packs

- Confidence: plausible. Horizon: later.
- Motivation: one substrate, N data-defined packs. Evidence: ADR-024, `docs/vision/02-shape.md`.
- Do not build: any pack without its consumer.
- Preserve: the admission case names a real corpus, eval fixtures, and a kill criterion.
- Promote when: per pack, a real consumer with a real corpus and eval fixtures.
- Drop if: a pack sits at 0 production claims 30 days after its compile (`docs/vision/11` A4).

## F7. The `ai-derived` evidence tier

- Confidence: speculative. Horizon: someday.
- Motivation: the first non-citable claim kind. Evidence: ADR-016.
- Do not build: the tier. Preserve: `tier_profile` column (ADR-017); tiers declared in CUE.
- Promote when: a non-citable claim kind is proposed with its own eval. Drop if: never needed.

## F8. Hosted read-only expert host (law 9)

- Confidence: plausible. Horizon: later.
- Motivation: hardened `serve-http` with auth and a read-only mode is the hosted surface,
  not a new server. Evidence: `docs/vision/12` §3, `docs/plans/gke-module-host.md`.
- Do not build: a writable hosted surface, tenancy, billing.
- Preserve: `Server.strictSurfaces` (ADR-014 amendment) as the hard gate for that shape.
- Promote when: the auth + read-only rows are proven and a first internal consumer session is stamped (law 6).
- Drop if: no external request in writing (`docs/vision/07` R1).

## F9. Machine-evaluated plan triggers

- Confidence: speculative. Horizon: someday.
- Motivation: `trigger:` on deferred rows is recorded, never evaluated, in v0. Evidence: ADR-018 "Rejected".
- Do not build: a trigger engine. Preserve: `trigger:` stays free text.
- Promote when: the same trigger vocabulary repeats across two repos' plans.
- Drop if: `needs:` eligibility and threads already answer the question.

## F10. CI fails closed (`--strict` with its own store)

- Confidence: likely. Horizon: next.
- Motivation: CI owns determinism; "daemon down" is not a valid CI state.
  Evidence: `docs/design/coding-agent-excellence.md` §5.3, `scripts/ci.sh`.
- Do not build: a CI service or runner config before the claims are reconciled.
- Preserve: `verify --store <tmp>` keeps working; ambient-fathom's claimed-Unknown rows
  are reconciled first (`th-c214dd26`).
- Promote when: `th-c214dd26` is decided and scope G6 is done.
- Drop if: the repo stays without a CI runner by decision.

## F11. Adopt the kit's `mcp-server` under `internal/mcp`

- Confidence: speculative. Horizon: someday. Evidence: ADR-022.
- Do not build: a wrapper layer. Promote when: the kit gains surface/group/prompts/resources support.
- Drop if: never.

## F12. An LLM-judged evaluation lane on `qa-harness`

- Confidence: speculative. Horizon: someday. Evidence: ADR-023, FA-12.
- Promote when: a second consumer needs LLM-enriched artifacts graded. Drop if: never.

## F13. Vectors as an additive in-store column

- Confidence: speculative. Horizon: someday.
- Motivation: hybrid semantic search; "evidence-gated exactly as designed", written for
  DuckDB VSS and never re-checked on SQLite. Evidence: ADR-016, `docs/design/product-vision.md` H3 note.
- Do not build: a second store. Promote when: a query class fails keyword-only on the context bench.
- Drop if: harness-native embeddings make it moot.

## F14. CI/forge integration as its own repo

- Confidence: plausible. Horizon: later. Evidence: `docs/handoff/repo-strategy.md` §2.
- Do not build: the repo. Preserve: the seam is a subcommand that takes a diff, calls
  fathom, posts results; review output stays verb JSON.
- Promote when: the first non-fleet consumer needs it, or its release cadence diverges.
- Drop if: lefthook remains the only delivery.

## F15. A second user or second machine

- Confidence: plausible. Horizon: later.
- Motivation: every instrument assumes one person, one machine, one loopback daemon.
  Evidence: `docs/vision/20` §7 Q1.
- Do not build: tenancy, auth, sync.
- Preserve: service-scoped exclusions; absolute store and call-log defaults (O11/O12 fixes);
  portable stores; ledgers keyed by something other than `$HOME` when touched.
- Promote when: a colleague adopts, as one deliberate hostile test.
- Drop if: single-user is accepted in writing (see `questions.md` Q5).

## F17. A saved-query discovery router

- Confidence: speculative. Horizon: someday. Evidence: declined in `docs/vision/24`.
- Listed to stop re-proposal. Promote when: `fathom prime` and the opening-moves table
  measurably fail to route discovery. Drop if: never asked for again.
