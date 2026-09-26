# 23 — The doctrine plane: every governing artifact earns its verb

**Status:** accepted 2026-08-13 — it landed as the custodian's spec with vision
22 (commit 8e3ee92c); §2's promotion pipeline and goals-as-bets are ADR-026's
2026-09-25 amendment (DP-3, DP-4). Plan: `docs/plans/custodian.md`.

> Owner asks, 2026-08-13: *"Does fathom work as a progress tracker too for
> implementation? … We can add the missing UI dashboards to SDD — make the
> UI earn its place. Should we elevate ADRs and VISION and GOALS to first
> class citizens? What about ENGINEERING_PRACTICES — something global and
> locally overrideable? Cuelang catalogs for any of this? Tie-ins to the
> UI?"* — with the standing instruction to go bigger where bigger is real
> and gate what is not.

## 1. The frame this belongs in

Fathom's arc is a sequence of planes becoming verifiable:

| Plane | Artifact | The verb it earned |
|---|---|---|
| Code | symbols, calls | the index (asked, not grepped) |
| Plans | `docs/plans/*.md` | **done means proven** (`fathom verify`) |
| Config | CLAUDE.md / AGENTS.md | **instructed means true** (vision 22) |
| **Doctrine** | ADRs, visions, practices, bets | **this document** |

The doctrine plane is the artifacts that *decide* — why the code is shaped
the way it is, what was promised, what is mandatory everywhere. They rot
exactly the way config rots: an accepted ADR nobody re-checks becomes a
confident false claim with a decision record's authority behind it. The
elevation rule, applied uniformly and without exception:

> **An artifact class becomes first-class by carrying falsifiable claims.
> First-class = identity + claims + a checker + a surface. What cannot
> carry a checkable claim stays prose — honored as context, never
> "elevated" into a system that would only decorate it.**

That rule is what keeps this from becoming a documentation CMS.

## 2. The gate on the asks

| Ask | Verdict |
|---|---|
| Fathom as implementation progress tracker | **ALREADY TRUE — the ask is the missing surface, accepted.** `fathom plans status` / `plans next` / `verify` compute SDD progress from files + index; nothing is asserted. What does not exist is the observatory panel. Accepted with the earn-its-place gate in §4. |
| ADRs first-class | **ACCEPTED — the strongest candidate: "decided means honored."** An accepted ADR is a standing claim about the codebase ("the store is pure Go", "no Node anywhere", "org-agnostic — zero references, hard requirement"). Those are *precisely* concern-shaped: forbidden patterns, required properties, checkable against the index. Elevation: an ADR may carry machine-checkable assertions; `fathom adr audit` reports accepted-but-violated decisions. An ADR that cannot carry one is still a valid ADR — its context and consequences are prose; it simply has no audit row (law 3: named absence, not a fake pass). |
| VISION first-class | **REFRAMED — visions are promises, not claims; their truth lives in the future.** Conformance-checking prose is theater. What IS checkable is the promotion pipeline this repo already practices: vision → design doc → plan rows → proven code. Elevation = pipeline tracking: every vision names its status/trigger or its plan rows; links resolve; staleness is dated. The surface is the pipeline view — where each promise stands, computed. |
| GOALS first-class | **GATED HARD — a goal without an instrument is OKR theater.** Fathom already has the honest shape of a goal: the **pre-registered bet** (the arm bench, `threadBetCheck`, every vision's falsification section — each names an instrument, a threshold, and a consequence). GOALS elevate only as bets: instrument + threshold + review date, refused at load otherwise — the uncheckable-concern refusal applied to ambition. No GOALS.md of adjectives will ever be a fathom artifact. |
| ENGINEERING_PRACTICES, global + locally overrideable | **ACCEPTED as content, REFUSED as a fourth registry.** The trap: obligations (edit-time rules), concerns (config audits), ADR assertions (conformance) — adding a parallel "practices" registry would make four overlapping rule systems, and overlapping registries are the config-rot generator one level up. Design: **one doctrine catalog** (CUE), where each entry declares which enforcement surface(s) it compiles to — obligation, concern, conformance. A "practice" is a catalog entry; an ENGINEERING_PRACTICES.md, where wanted, is a *render of* (or is audited against) the catalog — never a hand-maintained parallel truth. Global at `~/.fathom/` as user-scope defaults, repo `.fathom/` governs: law 13's shape, third use, machinery already built. |
| CUE catalogs | **YES for the contract layer, NO for the prose.** CUE holds identity, claims, checks — closed structs, `?:` refused, cue-vet in the gate (all house style already). Markdown holds the why. The catalog is the source of truth for what is *enforced*; documents remain the source of truth for what is *meant*. |
| UI tie-ins | **YES, one panel per instrument, none for prose** — see §4. |

## 3. What else earns elevation (asked-for bigger)

- **The laws themselves.** This program's numbered laws are doctrine-catalog
  entries avant la lettre — several already have mechanical teeth (law 3 in
  `stat.Absent`, law 15 in marker ownership). The catalog gives them
  identity and lets a repo cite them by ID.
- **Runbooks / operational doctrine.** O25 was runbook rot: a deploy
  instruction whose truth expired took the daemon down fleet-wide. A
  runbook's commands can be checked to exist (`command_documented_and_exists`
  is already a concern kind); its paths can be checked to resolve. Elevation
  is cheap and the failure it prevents is an outage, not a style nit.
- **Not elevated:** README (for humans; pointer-audited at most), CHANGELOG
  (append-only history — already honest by construction), design docs
  (bound to plans, which are already gated).

## 4. The SDD surface — how the UI earns its place

The observatory gains one **SDD panel** (fleet and per-repo), governed by
three rules that are the price of admission:

1. **Everything computed at render, nothing asserted.** The panel calls the
   same engine as `plans status`; a hand-maintained status row is a defect
   by definition.
2. **The headline is the claimed-vs-proven gap** — rows checked `[x]` that
   fail verification. It is the one number no conventional tracker has, the
   measured honesty of the checklist. Progress ("N of M proven, K open,
   J deferred-with-trigger") renders beside it; the gap leads.
3. **No velocity theater.** A trend chart requires a history instrument.
   The verify ledger (each `fathom verify` run appends {plan, counts,
   commit, at} under `workspace/`) starts recording when this ships; the
   trend chart appears only once the ledger has data to earn it — until
   then the panel shows `stat.Absent` with the reason, per law 3.

The doctrine plane adds rows to existing surfaces — ADR conformance in the
config panel's style, pipeline status per vision — and **no new realization
rung** (vision 22's rule holds).

## 5. What this is NOT

Not a documentation CMS. Not auto-generated ADRs. Not OKR software. Not an
LLM judging prose in a standing loop (the LLM lane stays behind the metrics
tiers, priced first). Not a fourth rule registry. Not a tracker that asks
humans to update status fields — fathom computes status or does not show it.

## 6. Falsification, pre-registered

- If ADR assertions turn out mostly inexpressible as mechanical checks
  (measured: assertions refused vs accepted at load), the assertion grammar
  is wrong — fix the grammar before reaching for an LLM lane.
- If the SDD panel draws no traffic within 30 days of shipping (the
  observatory counts page hits; `surface_traffic` is the precedent), it has
  not earned its place — remove it rather than promote it.
- If the verify ledger shows the claimed-vs-proven gap pinned at zero for 60
  days, the headline demotes to a summary row — a gauge that never moves
  teaches nothing (law 3 cuts both ways).
