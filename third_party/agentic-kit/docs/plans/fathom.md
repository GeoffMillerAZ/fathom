# fathom: what it gives this kit, and the kit's side of fathom's evidence plane

**Mission.** One place that says how agentic-kit uses fathom today, what the kit must honour
to stay compatible with fathom's evidence plane, and which opportunities that opens. Every row
below is open (`[ ]`) with a start trigger. Nothing is built by this plan, and a box flips only
when `fathom verify` proves it.

## 1. Summary

- **Today** fathom gives the kit a plan gate, a code index for where, who-calls and what-breaks
  questions, governing rules at the edit, intent and config audits, and SARIF with provenance.
- **Next** it will give fact sheets at a named commit, answers that carry their basis and fact
  receipts a stranger can check, served from index snapshots and a read-only environment daemon
  that serves prod's lineup.
- **The kit's part** is doctrine and shapes: how an agent reads and cites that evidence in an
  incident ([incident-readiness section 11](../incident-readiness.md)), and the evidence types
  fathom's receipts align with (`evidence-producer`, `evidence-span`).

**Source of truth.** This file holds the kit's use of fathom and the compatibility contract.
Asks from fathom to the kit stay in [2026-07-fathom-asks.md](2026-07-fathom-asks.md) (FA-1..10)
and [fathom-uplift-asks.md](fathom-uplift-asks.md) (FA-11..22). The incident doctrine lives in
[incident-readiness section 11](../incident-readiness.md); this file only maps it to fathom.
fathom's own records decide fathom's behaviour. They are cited as `fathom: path:line` at fathom
`1639b37c`. Kit facts are cited as `path:line` at `91268ac`.

**Names.** This file follows fathom's design (lands in fathom v0.11.0:
docs/design/index-snapshots.md), which retires "fathom workspaces":

| Term | Means |
|---|---|
| index snapshots | the capability: CI exports an index snapshot per service per commit; the operator publishes and signs it |
| index snapshot | one service's index at one commit, written `payments@d4e5f6`; immutable |
| lineup | a committed CUE file of lineup entries (`service@commit`) for one environment |
| environment daemon | a long-running `serve-http --read-only --lineup <file>`, deployed as `fathom-<env>` ("fathom-prod") |
| read-only sidecar | the same mode, run for one job (a review runner, an incident agent) over loopback |
| lineup change | a new environment daemon starts on the new lineup (a rolling update); there is no reload |
| answer basis | on every answer: `exact`, `differs`, `unchecked` or `uncommitted` (was "dirty"); `behind N` is computed by the caller only |
| fact sheet | per service at one commit: facts stored at ingest plus joins computed at request time, with receipts and coverage, no grades (was "concern sheet") |
| cache key, `--cache-from` ingest | what decides reuse of an index snapshot; starting an ingest from one |

Fathom's draft row ids below (`IXS-`, `WAE-`, `FG`) come from the records that land with that
design; `EVX-`, `IM-`, `VB-` rows are in fathom's tree today.

**Claim grades** follow [changes.md §10](../project-intent/changes.md): accepted, measured,
estimate, claimed, hypothesis, unmeasured, refused.

**Intent stated before design.** Decision 0004 and D13 admit no package here (no named
consumer). D10 governs absence claims; 0002 governs declared tool effects; 0006 links claims
to versioned evidence. Practices §17, §18, §20 and §27 apply. fathom ADR-063 O3 (kits never
build a second plan verifier) and O9 (separate evidence vocabularies, a naming audit only)
bind both sides (fathom: ADR.md:3979, ADR.md:3993-3995). No trigger fires from this plan.
Due elsewhere: 0010 revisits on 2026-10-12, 0011 reviews on 2026-10-16, and 0001 (proposed)
passed its 30-day window from 2026-08-27 without closing; none of them touches this design.

## 2. Today

| Capability | Command or tool | What the kit does with it | Grade |
|---|---|---|---|
| Plan gate | `fathom verify --service agentic-kit --plan docs/plans/<p>.md` | pre-push verifies every plan against a scratch ingest of the pushed tree (lefthook.yml:52-96). `--strict` makes an unknown claimed row block (fathom: docs/plans/README.md:37-38); `--format sarif\|check-run` renders refusals (fathom: cmd/fathom/verify.go:59). CI does not run it (FX-8) | accepted; every plan exit 0 at `91268ac` (ingest 43 s) and on this branch (ingest 164 s under load); neither is a committed reading |
| Row grammar after RF-5 | `reachable_from` passes on typed call edges only | never put `reachable_from` on a non-Go row; docs rows use `oracle: lexical` (example: decision-port.md:175) | accepted |
| Progress | `fathom plans status --service agentic-kit`, `fathom plans next` | computed progress and the next eligible rows | accepted |
| Where, who calls, what breaks | MCP `find_symbol`, `symbol_context`, `blast_radius` (the default eight, fathom: docs/mcp-tools/surfaces.md:47-50) | before changing an exported API, read its callers and reach, then run `task downstream-check` (AGENTS.md:12) | accepted |
| Diff review | MCP `review_diff`; CLI `fathom review --mode report --since <ref>` (never writes, fathom: cmd/fathom/review.go:55,62) | changed symbols with callers, governing rules, touched plan rows and the test gap, before a consumer breaks (FX-7) | unmeasured for the kit |
| Test selection | `fathom tests select --since <ref> --safe` (fathom: cmd/fathom/tests_select.go:81,87) | run the narrow set while iterating; `task validate` stays the gate | unmeasured for the kit |
| Certification gaps | `fathom certify gaps --json` (`fathom.certify.gaps/v1`, fathom: cmd/fathom/certify_gaps.go:33) | names targets no test certifies; it reads only declared certifications, and the kit declares none, so it answers empty today | claimed |
| Governing rules | `obligations` (no path: root conventions; a path: that file's rules), injected at the edit | the kit-self guards and the `agentic-kit-standards` layer (fathom-pack/README.md) | accepted |
| Intent audit | `fathom intent audit --json` (fathom: cmd/fathom/intentaudit.go:77) | checks `docs/intent/` conditions against the tree; FA-16 (open) asks `intent-doctor` and `intent-audit` to consume it | accepted |
| Config audit | `fathom config audit` over `.fathom/concerns.cue`; kind `stanza_tools_served` (fathom: internal/custodian/custodian.go:61) | would flag the stanza naming `references`, `outline`, `repo_map`, `pack_context` (AGENTS.md:92-93), extended-surface tools, not the default eight (FX-3) | hypothesis until registered |
| Receipts | `fathom receipt verify`: callers, references, symbol location and certification reach for Go from a bare checkout, offline (fathom: docs/plans/portable-receipts.md:32) | check a fathom answer about a kit symbol without trusting fathom's state | accepted |
| SARIF with provenance | seven callers through one emitter: verify, obligations, rules replay, config audit, intent audit, `review --receipt`, `review_diff` (fathom: docs/plans/portable-receipts.md:28-29) | a CI sink, when one is named | accepted; no kit sink |
| SCIP | `fathom export scip` (fathom: cmd/fathom/export.go:1) | symbols and typed edges for tools that read SCIP | no kit consumer |
| Alert or frame to code | `localize` (extended surface, Stage A) | rule:line, receiver, emitting log template, owners (an owners fix, IM-F1, is pending) | accepted; the incident section is decided, not built (fathom: ADR.md:3516) |

Not yet: `fathom export receipt` (EVX-9 open, fathom: docs/plans/portable-receipts.md:40);
`read_source` at a commit (it reads the working tree, fathom: internal/mcp/readsource.go:452);
the answer basis; fact sheets; index snapshots; the environment daemon.

## 3. Compatibility with fathom's evidence plane

### 3.1 The effort, in five lines

CI exports an index snapshot per service at each merge. The operator's CI publishes and signs
it; fathom never uploads, fetches or signs (fathom: docs/intent/invariants.md:85, I56). An
environment daemon serves the index snapshots its lineup names, read-only; a lineup change
starts a new daemon. Every answer carries an answer basis, and facts and joins export as
receipts with anchors and coverage. A review tool judges; fathom never grades, never holds the
findings record (fathom: ADR.md:2155, ADR-040) and never acts in production (fathom:
ADR.md:3573-3578, IM-X3, IM-X9). Status: design; it lands in fathom v0.11.0
(docs/design/index-snapshots.md). The owner-architect accepted its decisions D4 to D10, D12,
D13, D15 to D17 and D19 on 2026-10-11; the hosted parts wait on the owner's question U1.

### 3.2 The contract

| Element | fathom's shape | The kit's rule |
|---|---|---|
| Anchor | `{path, commit, blobId, start, end, excerptSha256, digestScope, state}`; state `committed\|uncommitted\|filtered\|withheld` (fathom: ADR.md:4561; internal/receipt/anchor.go:33-50) | cite the anchor whole; a `withheld` anchor has no excerpt and no digest and is never reconstructed |
| Coverage | `{status: complete\|incomplete, extractor, tier, readSet, blindSpots[]}` on every fact | an absence with `incomplete` coverage is a named lower bound (practices §17, `telemetry/read-manifest`; D10) |
| Re-derive level | `git`, `toolchain:<name>` or `fathom@<version>`, from a closed table, never the producer's say-so (fathom: ADR.md:4568) | never upgrade it; name it in a write-up so a reader knows who can check |
| Answer basis | `exact`, `differs`, `unchecked`, `uncommitted`; `behind N` caller side | section 3.3; a deploy gate accepts only `exact` |
| Index snapshot, lineup, environment daemon | immutable files; `serve-http --read-only --lineup` | a kit agent never asks the environment daemon to ingest, refresh or compile |
| Declared effects | the daemon refuses writes in read-only mode | a kit-built agent registering fathom tools declares queries `read` and `ingest` a write (decision 0002); a declared read is a claim until fathom's hostile read-only sweep (IXS-M3) reads clean (§18) |
| Version pin | the cache key includes the fathom version and extractor epoch; reuse hits only on the same release; env-kit's `env.lock` pins it | the kit's gate takes fathom from PATH, `FATHOM_BIN` or a sibling build with no pin (lefthook.yml:54-72); pin before any `--cache-from` (FX-9) |
| Fail-open | exit 2 is operational; a deploy gate (the review tool's admission, never a fathom verb) admits and records `evidence: unavailable` | the kit gate already fails open on exit 2 (lefthook.yml:91-94); say it by name, never silently (§20) |
| Producer | the planned receipt record carries "a producer in agentic-kit's `evidence-producer` shape (kind deterministic, origin self)" (fathom: docs/plans/portable-receipts.md:52) | `Origin` is relative to the consuming store and `Catalog.Admit` refuses `self` (foundations/evidence/evidence-producer/types.go:83-96,260-265); see Q9 and FX-2 |
| Boundaries | fathom ADR-063: no second verifier, result format or package graph | the kit builds no plan verifier (O3), no receipt or result format for code facts, no package graph; it keeps its own evidence types (O9) |

### 3.3 Answer basis handling

The doctrine and its table are [incident-readiness section 11](../incident-readiness.md),
point 2. In fathom terms: `exact` only when the caller passed the deployed commit and the index
snapshot holds it; `differs` when the caller passed a commit and the answer read another one;
`unchecked` when the caller named none, the usual case for an agent over MCP; `uncommitted`
only from a developer's local daemon. No basis ships yet, so read every answer fathom serves
today as `unchecked`. Until `read_source` takes a ref, read text at a commit with
`git show <commit>:<path>`.

### 3.4 Anchor mapping to `evidence-span`

fathom aligns its anchor with a review tool's `SpanRef` "and its planned `evidence-span`
successor" (fathom: ADR.md:4602-4603, ADR-068 decision 12). The fit today:

| fathom anchor | `evidence-span` field | Fit |
|---|---|---|
| path, start, end | `Locator{Kind: "line", Value: path, Start, End}` (foundations/evidence/evidence-span/types.go:43-64) | fits |
| excerptSha256 | `RecordHash`, 64-hex sha256 | fits the format; differs in meaning (the record, not the cited lines) |
| blobId | `RecordID` | fits as an identifier |
| commit | none (`SourceID` names a source-registry entry) | gap |
| state | none | gap: a `withheld` or `uncommitted` anchor is indistinguishable |
| producer | `Author` is `human` or `ai` only (foundations/provenance/ai-provenance/types.go:30-32) | gap: a deterministic tool's fact cannot pass `ValidateAsEvidence` honestly |
| tier | `Grade`, capped by the producer's `TierCap`; `DefaultKindCaps` caps a deterministic producer at pre-scan | open question 3 |

The type change is future F20, not this plan.

### 3.5 Routing questions to fathom

| Question | Tool | Not when |
|---|---|---|
| Where is X defined | `find_symbol`; an exact hit is the answer | you hold a literal string: `search` |
| Who calls X, briefly, which tests reach it | `symbol_context` | you need the full caller graph: `find_callers` (extended surface) |
| What breaks if X changes | `blast_radius` | the dependent service is not in this daemon's store: its index snapshot must be imported first (IXS-7, not built) |
| What a diff does | `review_diff`; CLI `fathom review --mode report --since <ref>` | |
| What governs this file | `obligations` with the path; root rules with no path | |
| Is this row proven | `verify_plan`; CLI `fathom verify` | never tick a box on a claim |
| Text at a line | `read_source`; at a commit, `git show` until the read at a ref ships (WAE-3) | the answer must describe prod and the read is the working tree |
| A literal, a config value, a log line, a doc contradiction | `search` | |
| An alert name or a pasted frame | `localize` (extended surface) | in a hook, an absent line is not evidence |

Do not route to fathom: runtime state (logs, metrics, cluster state, flags), since fathom never
fetches (I56); cause, severity or mitigation, which are judgment (IM-X9); any write against an
environment daemon.

### 3.6 fathom's skills in this repository

The `fathom-*` skills here are generated and fathom-owned (marker at
.claude/skills/fathom-activate/SKILL.md:11; source `dist/agents/skills/<name>/SKILL.md` in
fathom). Change them in fathom, never here. On 2026-10-11 all nine copies differed from
fathom's source at `1639b37c`, and `fathom-findings` was absent (FX-1). What fathom's sources
should gain once the pieces ship (asks to fathom, not kit work):

- `fathom-activate`: an evidence-plane step, run only after the pieces exist: pin fathom in
  `env.lock`; add `--expect-index-watermark <sha>` (fathom: cmd/fathom/review.go:64) to review
  gates; make `review --since <merge-base>` the PR ratchet; register a read-only sidecar for
  incident sessions; import dependency index snapshots.
- `fathom-custodian`: suggest `stanza_tools_served` by default, since this repository shows the
  drift it catches; a concern that incident instructions keep the answer-basis rule and "silence is
  not evidence".
- A responder binding (FX-4) that implements section 11 with section 3.5's routing, under
  proposed decision 0019.

### 3.7 How changes reach fathom

fathom vendors this kit at `9ecaf134` (fathom: go.mod:9, third_party/agentic-kit/.vendor-commit)
with `replace => ./third_party/agentic-kit` (fathom: go.mod:59). `task sync-kit` archives the
whole tree, docs and `.claude/skills` included (fathom: Taskfile.yml:321-325), and
`vendor-check` asserts the mirror matches the pin (fathom: Taskfile.yml:331-333). So this file
and section 11 reach fathom sessions at the next pin bump and sync. Nothing in fathom has to
change for a docs-only kit change to land. The vendored `fathom-*` skills are the same stale
copies, so FX-1 fixes both after that sync.

## 4. Tasks: opportunities, ranked

- [ ] **Refresh the generated fathom skills and add `fathom-findings`.** Why: all nine copies
  differ from fathom's source, so kit sessions follow an older fathom. Size: XS. Start: the
  owner commits the pending stanza v3, so both generated surfaces refresh in one owner pass
  (`fathom init --skills`). Kill: none {id: FX-1, oracle: lexical, path: .claude/skills/fathom-findings/SKILL.md, pattern: "^name: fathom-findings$"}
- [ ] **Ask fathom: the receipt record states its producer without a consumer-relative
  origin.** Why: a kit catalog that copies `origin: self` refuses fathom's facts as circular.
  Size: XS, a note on fathom's EVX-16. Start: before fathom builds EVX-16 (its wave 3, gated on
  EVX-G3). Kill: Q9 is answered another way {id: FX-2}
- [ ] **Register `stanza_tools_served` in `.fathom/concerns.cue`.** Why: `fathom config audit`
  then catches a stanza naming tools the default surface no longer serves. Size: XS. Start:
  FX-1 lands, so the concern starts green. Kill: two false violations in 30 days {id: FX-3, oracle: lexical, path: .fathom/concerns.cue, pattern: "stanza_tools_served", needs: "FX-1"}
- [ ] **The incident-responder binding.** Why: section 11 without a binding goes unused, and
  the binding must move with fathom's tool names. Size: M on fathom's side, XS here (a link).
  Start: the owner accepts decision 0019, and fathom serves the answer basis and one service's fact
  sheet (its FG0 gap and IXS-2), or fathom's IM-M0 reads BUILD. Kill: no incident or replay
  session recorded with fathom's law 6 stamp file within 60 days of the binding shipping {id: FX-4}
- [ ] **An anchor mapping note in `foundations/evidence/README.md`.** Why: a consumer importing
  fathom receipts into kit spans needs section 3.4's mapping and its gaps, and the gaps decide
  F20. Size: S. Start: fathom ships `export receipt` (EVX-9), so the note is checked against a
  real receipt. Kill: none {id: FX-5, oracle: lexical, path: foundations/evidence/README.md, pattern: "fathom receipt"}
- [ ] **Offer one kit package for fathom's stranger test.** Why: EVX-M1 names "one agentic-kit
  or app-kit package" (fathom: docs/plans/portable-receipts.md:45); the kit becomes a receipt
  subject and a candidate reader for EVX-G3. Size: XS. Start: fathom's EVX-10, EVX-12 and
  EVX-13 ship. Kill: EVX-M1 reads CUT {id: FX-6}
- [ ] **Report-mode review and safe test selection beside `downstream-check`.** Why: shows
  changed symbols, callers, governing rules and the test gap before a consumer breaks. Size: S.
  Start: the owner approves the Taskfile change (a gate edit; autonomy.md asks first). Kill:
  no actionable finding over 10 exported-API changes {id: FX-7, oracle: lexical, path: Taskfile.yml, pattern: "review --mode report"}
- [ ] **CI runs `fathom verify --strict --format check-run` over `docs/plans/`.** Why: only the
  local pre-push verifies today, and CI never sees an unknown claimed row. Size: S. Start: a
  pinned fathom release binary CI can install without fathom fetching anything. Kill: none {id: FX-8, oracle: lexical, path: .github/workflows/ci.yml, pattern: "fathom verify --strict"}
- [ ] **Pin fathom and start the gate's scratch ingest with `--cache-from`.** Why: each push
  pays about 43 s of scratch ingest (estimate from one scan), and reuse hits only on a pinned
  release. Size: S. Start: fathom ships `--cache-from` (IXS-1) with parity proven (IXS-M0b), and
  an index snapshot of this kit's `main` exists. Kill: the median over 20 pushes is above 0.6x
  the plain scratch ingest {id: FX-9, oracle: lexical, path: lefthook.yml, pattern: "--cache-from"}
- [ ] **A responder blueprint under `scenarios/`.** Why: proves decision 0002's declared
  effects and section 11 end to end: an agent on `kits/agent-harness` reads a read-only sidecar,
  queries declared `read`, `ingest` refused. Size: M. Start: F19 promotes and fathom ships the
  read-only sidecar. Kill: no named consumer within 90 days {id: FX-10, needs: "FX-4"}
- [ ] **A git-native anchor on `evidence-span`.** Why: optional commit, blob id, anchor state
  and a deterministic author let a fathom fact pass `ValidateAsEvidence` honestly. Size: M.
  Start: F20 promotes (two consumers, D13). Kill: F20's drop condition {id: FX-11, needs: "FX-5"}

### Refused, with the reason

| Idea | Why refused | What would reopen it |
|---|---|---|
| A kit-owned skill naming fathom's tools | goes stale with fathom's surface (proposed 0019) | 0019's revisit conditions |
| A kit receipt or result format for code facts | fathom ADR-063: no second result format | ADR-063 revisited |
| A kit plan verifier | ADR-063 O3 | never |
| Merging the kit's evidence vocabulary into fathom's | ADR-063 O9 keeps separate types | the naming audit (K10) lands |
| A kit package for the fact sheet or the answer basis | no named consumer (0004, D13) | F19 promotes |
| A kit tool that mitigates, runs probes, fetches or names a cause | practices §27, guide section 8 | the guide's own reopen triggers |
| Hand-editing the generated stanza (AGENTS.md:87-118) | generated; the owner holds stanza v3 | never; regenerate |

## 5. What the kit gives fathom back

- The `evidence-producer` shape for the producer in fathom's planned receipt record (EVX-16),
  and with it the tier cap and the circular-origin refusal (Q9).
- The incident practice, generalized from fathom's ADR-058 into practices §27 and the guide,
  and now section 11's evidence-before-claim doctrine.
- `quality/structural-refusal` and `telemetry/read-manifest`: the shapes for `unknown` and
  lower bounds (FA-20, FA-21, closed).
- Decision 0002's declared effects: the vocabulary for read-only use of fathom's tools.
- The project-intent practice and `intent-*` skills, the fathom-pack guards and layer, and the
  decision port, all vendored into fathom (section 3.7).
- A receipt subject for EVX-M1 and a candidate reader for EVX-G3. The kit is owner-written, so
  it counts toward EVX-G3 only beside a second consumer.

## 6. Open questions for the owner

1. Decision 0019 (proposed): does the responder binding live in fathom, with the doctrine here?
2. Q9: how does a receipt state its producer's origin?
3. Tier for an imported fathom fact: `DefaultKindCaps` caps a deterministic producer at
   pre-scan. Does a fact whose anchor the consumer re-checked at the git rung count as verified?
4. The pin: adopt env-kit's `env.lock` in this repository, or pin through `FATHOM_BIN` in
   `lefthook.yml` (FX-9)?
5. Should the kit be named as a reader for fathom's EVX-G3 (FX-6)?
6. fathom's U1 (a self-hosted, single-organization, read-only environment daemon) gates FX-10;
   the kit has no position beyond that.
