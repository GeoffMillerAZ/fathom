# 12 — The governed knowledge engine: module host, expertchat, hosted chat

**Status:** accepted 2026-07-28 — the owner decision of §1 pulled phases G–I
forward that day. Phase F is landed (the store-format spec and the research
flywheel, on `main`); phase G became `docs/plans/gke-wave1.md` and
`docs/plans/gke-module-host.md`; phase H is agentic-kit's (`expertchat`); phase I,
the hosted chat path, is held by two standing non-goals in `docs/intent/scope.md`
(no hosted multi-tenant expert backend, no first-party chat) until their Revisit
conditions hold. Restated 2026-09-25.
Plans: `docs/plans/gke-wave1.md`, `docs/plans/gke-module-host.md`.
**Successor to** `11-dial-to-eleven.md` (Phases A–E complete or in flight, K
in flight) — this doc does not re-litigate that program; it governs what the
just-landed spec and flywheel unlock next.
**Inputs:** the merged store-format v0 + expert-module v0 specs
(`docs/spec/`), the merged research flywheel (`docs/design/
research-flywheel.md`), `docs/design/product-vision.md` §2/§4 (H2/H3), and
an owner decision (2026-07-28, load-bearing, see §1) to pull the multi-expert
chat capability forward from H3 ahead of its written entry condition.

## 1. The framing: the store format is no longer a phrase in a roadmap doc

`docs/design/product-vision.md` named the durable asset in one sentence: *"a
versioned, self-describing store format third parties can target without us
in the loop... becoming a platform is an event — a third party targets the
format — not a roadmap phase."* Until 2026-07-28 that was a claim with no
falsifiable shape. It now has one:

- **`docs/spec/store-format-v0.md`** is normative on the on-disk SQLite
  format — file container, the two format-identity pragmas (`application_id
  = 0x46415448`, `user_version = 1`), required tables per conformance level
  (L0/L1/Writer/Server), anchor rules, watermark semantics, the closed
  evidence-tier vocabulary, and claim-lineage rules including refutation
  durability. Every MUST carries a named test or an explicit **UNENFORCED**
  pointer — `internal/store/format_v0_test.go`'s `TestStoreFormatV0_
  RequiredSchema`, `TestStoreFormatV0_StampsUnversionedStore`, and
  `TestStoreFormatV0_DocMatchesGoSource` are the falsifiability instruments;
  the last one diffs the doc's own required-schema tables against the live
  Go DDL, so the spec cannot drift from the binary that implements it
  without a failing test.
- **`docs/spec/expert-module-v0.md`** defines a module as *one conforming
  store + one manifest* — nothing else, no second engine
  (`schema/module/schema.cue`'s `#Module`, tested by `internal/modspec/
  schema_test.go`). The freshness doctrine (a module without a watermark is
  invalid) and the portability contract (compile on machine A, serve on
  machine B, zero recompile — `pkg/client/portability_test.go`'s
  `TestStorePortability_RoundTrip`, and the non-git-corpus instance,
  `TestExpertModule_NonGitCorpus_EndToEnd`) are both proven in code today,
  not aspirational.
- **The research flywheel** (`docs/design/research-flywheel.md`) closes a
  different, load-bearing gap: doctrine law 6 ("activation is stamped,
  never prose-documented") had no instrument of its own. `fathom research
  report` aggregates fathom's own pre-existing instrumentation (call log,
  hooks beat log, observations, bench artifacts) into one honest,
  never-fabricates-a-zero snapshot; `fathom research bundle` is the
  sanitized, opt-in, review-first twin — salted-hash service identity, a
  closed CUE schema (`schema/research-bundle/schema.cue`'s `#Bundle`)
  enforced twice (Go-side typed fields with no free-text field, CUE-side
  closed struct), adversarially tested against hostile input
  (`internal/research/bundle/adversarial_test.go`). No new MCP tool —
  doctrine 4 held even here.

**What this wave is not.** Landing a spec is not landing a platform. Nobody
outside this repo has targeted the format yet — the H3 entry condition for
federation (`docs/spec/store-format-v0.md` §11: "≥2 real external module
consumers, or ≥100 external installs + ≥60% internal fathom-share sustained
4 weeks") remains **unmet, at zero**, exactly as `product-vision.md` §4 H3
states it. This doc does not claim otherwise.

**The owner decision this doc carries forward, stated as what it is.** The
owner has chosen to pull the **module host** (read-only federation over N
conforming stores) and the **multi-expert chat** capability
(`product-vision.md` §4 H3: *"multi-expert systems ('chat with expert
modules')... orchestration composes above the engine over federated
stores"*) forward from H3, ahead of its external-adoption entry condition,
scoped **internally** — across the fleet's own conforming stores
(fathom, agentic-kit, app-kit, throughline), not for an external audience.
This is a deliberate schedule override, not a claim that the evidence bar
fired. It is recorded here, not silently, because doctrine law 8 ("kill
clocks require a consumer population") exists precisely to prevent
build-ahead-of-evidence from masquerading as demand-driven work. The
justification is different from a demand signal: the spec now exists, the
fleet already has ≥4 conforming stores sitting on one machine, and
`expertchat` over the fleet's own governed knowledge is the dogfood loop
that would generate the external-adoption evidence H3 actually asks for —
building it is how the entry condition could ever be reached, not a bypass
of it. Phases G–I below are gated on their own kill criteria regardless of
this override (§3): the override licenses *starting*, not a free pass on
*succeeding*.

## 2. Doctrine (carried in full, one law added)

The eight laws of `docs/vision/11-dial-to-eleven.md` §2 govern this wave
unchanged — governed evidence is the moat; determinism first; never silent;
consolidate don't grow; minimal theater; activation is stamped, never
prose-documented; instruments get existence checks; kill clocks require a
consumer population. Two of them bite hardest here: **law 4** (module host
and chat must not grow the MCP tool surface without the same forced-channel
or measured-evidence justification `read_source` earned — see §3's surface
design) and **law 8** (§1's override does not suspend the requirement that
an *internal* consumer population — fleet sessions actually using module
chat — exists before any adoption-shaped claim is made about it).

9. **NEW — A hosted surface ships read-only before it ships writable.**
   `store-format-v0.md` §1.3 already documents the reference implementation's
   own honest gap: there is no pure-read code path today, `store.Open` always
   runs `createSchema`. Before any hosted deployment of the module host or a
   chat surface accepts network-reachable traffic, that reachable surface
   MUST run in a mode that cannot write to a module store it did not create
   itself — auth and read-only enforcement are launch requirements for
   *hosted*, not later hardening. This is the same discipline that already
   governs the local daemon (`serve-http` is loopback-only, unauthenticated,
   by design, per `docs/mcp-tools.md`) restated for the case where "loopback
   only" stops being true.

## 3. The architecture decision (normative)

Stated plainly, so `docs/plans/gke-module-host.md` and `docs/plans/
gke-expertchat.md` have one sentence to build against each:

**The module host is `fathom serve-http` mounting N conforming module
stores read-only, as one surface.** Not a new binary, not a second engine —
the same discipline `docs/design/expert-modules.md` §1 states for the local
daemon ("there is no second engine, no second schema") extended across
store boundaries. Mounting is `ATTACH`-shaped (a multi-store query layer
behind `pkg/platform`'s existing cross-service graph machinery —
`pkg/platform.Build`/`BuildFromStore`, the same primitive
`CrossRepoBlastRadius` already generalizes over N services' stores) — not a
new federation engine, and explicitly the mechanism `store-format-v0.md`
§11 names as living "at a layer *above* this format, not a feature of the
format itself." Every module mounted this way is conformance-checked against
`store-format-v0.md`'s file-identity markers (§2: `application_id =
0x46415448`) at mount time, never silently accepted — a store that is not a
conforming fathom store, or is conforming but below the level the host
requires, is refused loudly (`docs/plans/gke-module-host.md`'s conformance
row). Every claim answered through the module host carries the same
per-module identity, evidence tier, and staleness that `store-format-v0.md`
§3's Server conformance level already requires of a single store — federation
adds a module identity to the payload, it does not relax any existing
honesty requirement.

**Chat is MCP-native.** There is no fathom-proprietary chat protocol. Any
MCP client — Claude Code, Claude Desktop, a future kit-built UI — is a
frontend to the same module-host surface. This keeps chat from becoming a
second serving path parallel to the one `docs/mcp-tools.md` already
documents; it is a *consumer* of the existing tool contract, composed
above it.

**The governed answer loop lives in agentic-kit, as an `expertchat`
package** (a kit, per the kit's own layering rule — `CLAUDE.md`
"kits compose foundations" — composing `foundations/llmclient` for the
model turn, `foundations/review/scoring` for the same evidence-tier
vocabulary the store format already closes over, `foundations/retrieval`
for the module-host query step, and `foundations/telemetry` for the
existence-check instrumentation law 7 already names). Its contract, stated
now so the kit plan (`agentic-kit the kit's docs/plans/gke-expertchat.md (moved 2026-07-28: a repo:agentic-kit plan verifies under the kit's own gate, not fathom's)`) does not have to
invent it later: **retrieve via the module host, then answer MUST cite
every claim with tier + staleness, and MUST refuse rather than answer when
retrieval returns no evidence.** This is D-EM4
(`docs/design/semantic-layers.md` §5, restated as a conformance requirement
by `expert-module-v0.md` §2) given to a chat loop instead of a lookup tool —
"why should this be trusted, when did it stop being sure, what was it
compiled from" answered on every turn, not just every `obligations` call.
`app-kit`'s `appkit/ui/chat` is the optional first-party UI for this loop —
optional because the loop's own contract is MCP-native and UI-agnostic; a
terminal MCP client with no UI at all is an equally conforming consumer.

**Hosted = hardened `serve-http`.** Not a new deployment target. The
existing daemon, given auth and a read-only mode (law 9), is the hosted
surface — `docs/plans/gke-module-host.md` carries the hardening rows,
`the kit's docs/plans/gke-expertchat.md` carries a thin hosted-chat-service example
that is a consumer of the hardened surface, not a new server.

## 4. The program

**(op)** marks owner-action items. Everything else is agent-executable, same
convention as `11-dial-to-eleven.md` §3.

### Phase F — Foundation (landed, retrospective)

| id | move | terminal state |
|---|---|---|
| F1 | Governed Store Format v0 + Expert Module v0 merged, critic-reviewed, falsifiability instruments passing (`docs/plans/gke-wave1.md`) | **done-when-stamped — stamped 2026-07-28, commit `90cee6a`/`37d1a75`** |
| F2 | Research flywheel merged, critic-reviewed, six findings closed (`docs/plans/gke-wave1.md`) | **done-when-stamped — stamped 2026-07-28, commit `e5828e0`/`90cee6a` lineage, `9edb41c`** |
| F3 | Live-store format-marker rollout: the fleet's `workspace/db/fleet.sqlite` gets `stampFormatMarkers` on its next writer open (a normal, non-eventful open — no operator step required beyond the daemon's own restart cadence) | **operator-observed, not yet confirmed against the live fleet store — the unchecked row in `docs/plans/gke-wave1.md`; done-when-stamped by a direct `PRAGMA user_version` read against the live file, not by code review of the write path (which is already covered by `TestStoreFormatV0_StampsUnversionedStore` against a synthetic store)** |

Full row-level evidence: `docs/plans/gke-wave1.md`.

### Phase G — Module host (read-only federation)

| id | move | kill criterion |
|---|---|---|
| G1 | Mount/unmount N conforming module stores read-only; conformance-checked at mount via the store-format-v0 markers (§2) | a mount path that silently degrades (serves a non-conforming or lower-conformance-than-required store without refusing) found in review or by test → treated as a law-3 ("never silent") violation, blocks the phase until fixed |
| G2 | Federated query layer with per-module provenance on every answer | a served claim missing its owning-module identity in the payload → same law-3 treatment as G1 |
| G3 | Surface exposure decision: extend `/rpc/agent`'s existing allowlist vs. a new scoped `/rpc/modules` surface | decided under doctrine 4 in `docs/plans/gke-module-host.md`'s design section — no tool ships until that argument is written down, mirroring `read_source`'s (T5-4) evidence-first precedent |
| G4 | Staleness honesty per module (each module's own watermark drift, not just the local store's) | a federated answer that reads fresher than its least-fresh contributing module → contradicts `store-format-v0.md` §6, blocks ship |
| G5 | Auth + read-only hardening for hosted deployment (law 9) | no hosted deployment ships before this row is proven — the phase's own gate, not a 30-day clock |
| G6 | Conformance refusal tests: a non-conforming store is refused loudly at mount, never silently degraded | the test suite itself is the terminal state — `docs/plans/gke-module-host.md` |

### Phase H — agentic-kit `expertchat` uplift (repo: agentic-kit, parallel track)

| id | move | kill / terminal state |
|---|---|---|
| H1 | `expertchat` package: governed answer composition over the module-host MCP surface | `the kit's docs/plans/gke-expertchat.md`; verified against the kit's own index once built, per-repo verify discipline (agentic-kit's `task validate`, not fathom's gate) |
| H2 | Citation-mandatory response type — a response with an unsourced claim is a type error, not a runtime check | same as H1 |
| H3 | Refusal-without-evidence behavior — zero retrieved evidence MUST refuse, never answer from model prior | kill: any code path that answers without a retrieval hit found in review → blocks merge, not a 30-day clock (this is a law-1/law-3 violation, not an adoption question) |
| H4 | Tier/staleness rendering contract — every citation surfaces its evidence tier and non-fresh status, mirroring `store-format-v0.md` §3 Server conformance at the chat layer | same as H1 |
| H5 | Thin hosted chat service example — a consumer of Phase G's hardened surface, not a new server | done-when-example-runs against a real (even single-module) module host |
| H6 | `app-kit`'s `appkit/ui/chat` wiring pointer — documented, not built here (app-kit is a separate repo/track) | done-when-documented; the UI itself is app-kit's own scope |

### Phase I — Hosted chat path (gated on G5 + H5)

| id | move | kill criterion |
|---|---|---|
| I1 | Hardened `serve-http` deployed with auth + read-only mode live (not just coded) | cannot start before G5 is proven; law 9 |
| I2 | Hosted `expertchat` service wired to the module host, using H5's example as its base | cannot start before H5 |
| I3 | First internal (fleet) consumer session against the hosted path, recorded per law 6 (a stamp, not prose) | zero qualifying sessions in the window law 8 would otherwise require for an adoption claim → the hosted path stays *unmeasured*, not killed — same discipline law 8 already prescribes; this phase does not get to claim success on log silence |

## 5. Anti-moves (carried, with two additions for this wave)

Everything `11-dial-to-eleven.md` §4 already rules out stays ruled out
(no natural-language prose reviewer, no public plugin SDK before a second
consumer, no hosted multi-tenant anything as a FATAL risk-register item —
**this still applies**: Phase I's hosted surface is one operator's fleet,
authenticated and read-only-hardened, not a multi-tenant product; nothing
in this wave revisits R1). Additions:

- **No writable hosted surface, ever, before G5/law 9 is proven.** Not a
  30-day clock — a hosted writable surface ships only after the read-only
  hardening exists and is verified, full stop.
- **No claim that the H3 federation entry condition has fired.** §1's
  override licenses building the module host and chat internally; it does
  not license writing "federation has arrived" anywhere in product docs.
  `docs/design/product-vision.md` §4 H3's entry condition (≥2 external
  module consumers, or ≥100 installs + ≥60% share) is unchanged by this
  wave and stays the bar for any external-facing federation claim.

## 6. Falsifiability of this wave

Phase F is already falsified-or-proven — it is retrospective, and
`docs/plans/gke-wave1.md`'s rows point at real, passing tests. Phases G–I
are not: the honest falsifiable claim this doc makes is narrower than
"federation works" — it is **"a fleet-internal module host can answer a
cited, tier-and-staleness-honest question spanning more than one of the
fleet's own conforming stores, refusing rather than fabricating when
evidence is absent, without growing the MCP tool surface beyond what
doctrine 4 already permits."** If G1–G4 and H1–H4 land and that composite
claim does not hold under review (a federated answer drops provenance, a
chat turn answers without citations, or the tool surface grows without the
doctrine-4 argument G3 requires), the correct response is the one
`11-dial-to-eleven.md` §6 already modeled: retract the claim in the docs
that asserted it, not quietly keep shipping. Phase I's own falsifiability is
narrower still and is stated in its own row (I3): a hosted path with no
qualifying internal session is *unmeasured*, per law 8 — this doc does not
pre-commit to declaring it either a success or a failure before that
evidence exists.
