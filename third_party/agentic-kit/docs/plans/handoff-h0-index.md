# H0 — Handoff index & orchestration handbook

**What this is.** The work-order package for completing agentic-kit's
end-to-end vision, written for autonomous agent teams: an **Opus
coordinator** per spec, running **Sonnet implementer teams**. The library
uplift is done (v0.2.1→v0.6.0, 2026-07-10, gate green, 174 packages); what
remains is the *proof layer* — downstream adoption, the pull queue, the
trigger-gated bench, and 1.0 candidacy. Each `handoff-h*.md` sibling file is
one self-contained spec. Specs are minimal and theater-free by construction:
every deliverable's "done" is machine-checkable, and this file's rules make
"looks done" structurally hard.

**North star (from `2026-07-uplift.md` §0):** every claim checked against
ground truth — the model's (structured outputs validated by CUE), the
agent's (evidence spans, grades, provenance), and the kit's own (a
constitution enforced by the gate, not by memory). Every spec serves it;
work that doesn't is out of scope by default.

## 1. The roster

Sizes and triggers below restate each spec's own **Trigger**/**Size & team**
lines; the spec file is normative. Table last verified against the repos
2026-07-16 (vision-fit audit — supersedes the 2026-07-15 pass).

| Spec | Mission (one line) | Trigger (verified 2026-07-16) | Size | Order |
|---|---|---|---|---|
| H1 `handoff-h1-proof-gate.md` | Fix fathom `verify` (4 root causes: index pollution, ambiguity poisoning, `package:` grammar gap, method-edge resolution) in ../sextant; re-arm the pre-push done-means-proven gate | **SATISFIED** — kit `85de56b` closes the tail (restart done, HTTP-mode 15/15); fathom-side fix `b7f6476` + O9 residue | M | **first — everything else's acceptance rested on it** |
| H2 `handoff-h2-pull-queue.md` | Close TRACKING pull-queue rows D35-1…D35-5 incl. throughline donor deletions | **SATISFIED** — kit `3ea7c55`, D35-1..5 all `[x]`/`[-]`; non-negotiable #6's blocking clause is cleared | L | second — doctrine blocked new packages until closed |
| H3 `handoff-h3-throughline.md` | Throughline adoption arc: guardrails chat, crypto-shred vault, case-file, document-domain Tier-2; full harness port trigger-gated (beware: throughline's "M18.1" tracker label names a DIFFERENT, done milestone — verify the code, not the label) | stages 0–4 **ready now** after Geoff commits its pending tree; stage 5 gated on a real multi-turn workload | XL (L w/o stage 5) | parallel with H4 after H2 |
| H4 `handoff-h4-sextant.md` | Sextant governance stack, vector-store adoption, ladder Tier-1 adapter (Stage 5 parked 2026-07-16) + gap-log flywheel | **NOT fired** — its `feat/mcp-scoped-surfaces` branch still mid-flight & dirty; wait for merge + clean tree | L | after H1 (same repo), once trigger fires |
| H5 `handoff-h5-eval-runner.md` | kits/eval-runner over the landed eval slice; judge stack + D7 as trigger-gated follow-ons | **Stage 1 done** (kit `149a570`); stages 2–3 parked (docs-only, verbatim triggers); **owner ratification open** (throughline-derived corpus redaction sign-off) | M | third, after H2 |
| H6 `handoff-h6-protocol.md` | MCP Streamable HTTP (subscriber: fathom's hand-rolled serve-http, absorbed) + cross-process trace (#21, topology exists) | **EXECUTED** (kit `9c5f6e0`) for both stages; prompts/resources correctly unfired | M | independent |
| H7 `handoff-h7-patterns-bench.md` | Bench of 9 subscriber-gated work orders (#2 #3 #4 #11 #13 #17 #22 #23, improvement-loop) | none fired by design; per-order trigger + Geoff-ratified pull-queue row is a blocking precondition. WO-4 (#11) adjudicated not-fired 2026-07-16 (substantive live-bulk-call condition unmet) | S bench; S–M per order | pick only when a trigger fires |
| H8 `handoff-h8-public-candidacy.md` | Mechanical 1.0/public readiness (hygiene stage A + publication stage B) | stage A **ready now** (valuable even if D4 = no); stage B **blocked on Geoff ratifying D4 = YES** | L | stage A anytime; stage B last |

Dependency spine (as authored): H1 → (H2 → H5), (H2 → H3 ∥ H4-when-fired), H6
independent, H7 by ratified trigger only, H8-B by D4 only. **Live graph as of
2026-07-16** (the first four nodes are satisfied, so the spine collapses to
its remaining open edges): H3-pending-Geoff (his pending tree commit),
H4-when-fired (sextant branch merge), H7-per-trigger (per work order),
H8-A-anytime / H8-B-on-D4. The pull-queue doctrine block (non-negotiable #6)
that used to gate new packages behind H2 is cleared.

## 2. Non-negotiables (the constitution, enforced)

1. **The gate.** `task validate` (fmt-check · vet · cue-vet incl. testdata ·
   `go test -race ./...`) **plus `constitution-lint`** must be green before
   any commit is called done. `task downstream-check` before touching
   exported APIs or tagging. Never `git commit --no-verify`; opt out only
   with `LEFTHOOK=0` and only if a hook is itself broken.
2. **Ledger in the same change.** Every capability commit updates TRACKING.md
   (+ CHANGELOG.md, + `task docs-index` regeneration). constitution-lint's
   tracking-coverage check fails the gate otherwise — this is by design, do
   not fight it.
3. **Done means proven.** Deliverables in these specs are `- [ ]` boxes,
   many with `{symbol: Name}` anchors. Mark `[x]` only when the gate is
   green; anchored claims are verified by fathom at pre-push (after H1
   restores it). Marking `[x]` on a red gate is the defect class VISION
   ranks equal to a failing test.
4. **Hard rules.** Stdlib-only root module (deps only in nested modules —
   see D1); CUE owns shape (schema.cue + testdata for data-shaped packages);
   injected clocks (`now func() time.Time`, documented nil-default only);
   org-agnostic (no company/product names, no hardcoded catalogs or model
   IDs — CUE data always); imports point downward; concurrency contract in
   every package doc; non-guarantees stated as plainly as guarantees.
5. **Junk gate.** A deliverable whose trigger has not fired is NOT built —
   verify the trigger first, record the verification. Speculative work is
   junk even when it sounds visionary.
6. **Pull-ordered queue.** While a TRACKING pull-queue row is open, no new
   zero-consumer package lands (doctrine amendment 2). H2 exists to clear it.

## 3. The wave process (proven on the uplift — use it verbatim)

- **Coordinator (Opus)** decomposes its spec into waves of **≤4 Sonnet
  implementers on disjoint file sets**. The coordinator alone runs the gate,
  commits, tags, pushes — implementers never commit.
- One **ledger agent per wave** keeps TRACKING/CHANGELOG/catalog current so
  constitution-lint stays green at every commit.
- **Verify file state, don't respawn blindly.** Implementer final messages
  sometimes cut off mid-narration while the work is complete — check the
  files and the gate before re-running an agent.
- **Trust the toolchain, not the diagnostics panel.** In-harness diagnostics
  are often stale mid-edit snapshots; `go build` / `go test -race` are the
  truth.
- **CUE gotchas:** fixtures need NAMED values (bare `_:` labels fail
  `cue vet`) and package-qualified imports (`import "…:pkgname"`).
- **Git:** conventional-ish subjects (`feat(foundations): …`); co-author
  trailer on agent-authored commits; push with `env -u GITHUB_TOKEN` (the
  ambient token is the wrong identity). Tag only at green gates. Sibling
  repos: **never** commit in ../throughline / ../sextant /
  ../growing-gardens-bi unless the spec's write-scope says so, and never
  touch files unrelated to the spec on a dirty checkout.
- **Cassettes:** if `foundations/llmclient` types change shape, regenerate
  `examples/agent-cli/testdata/mock-session.jsonl` (`go generate
  ./examples/agent-cli`) in the same change — the gate pins it.

## 4. Escalation to the owner (Geoff)

Stop and hand back — do not improvise — when: (a) a spec's trigger is
ambiguous or appears not to have fired; (b) work requires committing in a
sibling repo whose tree carries changes the spec didn't create (throughline
and growing-gardens-bi currently hold uncommitted adoption changes pending
his review); (c) a decision is marked D4/owner-ratification; (d) closing a
deliverable would require weakening a proof requirement; (e) two specs
collide on the same files. Everything else: decide, record the decision in
the spec file or TRACKING, proceed.

## 5. Reading order for a fresh coordinator

`VISION.md` → `AGENTS.md` → this file → your `handoff-h*.md` spec →
`TRACKING.md` (your area) → `docs/plans/2026-07-uplift.md` §3 (decisions
D1–D10) and §7 (merge-map) → the code your spec names. Total ≈ one hour of
context; do not start implementing before finishing it.
