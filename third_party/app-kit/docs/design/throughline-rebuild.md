# Throughline Rebuild — the validation protocol for north-star test #1

**Status:** Protocol (pre-inventory). This document is a *procedure with
blanks*, not an analysis. Throughline's source currently lives at
`/Volumes/m2ext/code/geoffmilleraz/throughline` and is unreadable from this
environment (macOS TCC denial on `/Volumes/m2ext` as of 2026-06-10; the
project is not on public GitHub). Every fact we do not yet have is marked
`[TO FILL: ...]`. The protocol is written so that the inventory pass can be
executed largely by agents the moment the volume becomes readable, with Geoff
adjudicating only the judgment calls.

**What we know for certain:** Throughline exists at the path above, predates
the Kit, and the goal — verbatim from [../../VISION.md](../../VISION.md),
north-star test #1 — is: *rebuilt on the Kit, feature parity, less
app-specific code, ships as web + macOS + Windows from one repo.* This
rebuild is Phase 4 of [../../ROADMAP.md](../../ROADMAP.md) (Gate G4).

**Why this app is the test:** a rebuild of a real, pre-kit app is the only
honest probe of the Kit's claims. Greenfield apps flatter a platform;
rebuilds interrogate it. Every place Throughline needs something the Kit
lacks is a requirements discovery, and the protocol below is designed to
harvest those discoveries deliberately rather than trip over them.

---

## 0. Precondition: making the source readable

Nothing else in this document can execute until this step completes.

1. Grant disk access to the terminal/agent host process: System Settings →
   Privacy & Security → Full Disk Access (or, narrower, Files and Folders →
   Removable Volumes) for the terminal emulator and any agent runner that
   needs to read `/Volumes/m2ext`.
2. Verify: `ls /Volumes/m2ext/code/geoffmilleraz/throughline` succeeds.
3. **Snapshot before anything touches it.** Copy the tree to a
   working location on the internal disk. If it is a git repo, also create a
   bundle: `git bundle create throughline-baseline.bundle --all`. Record:
   - `[TO FILL: baseline commit hash, or archive SHA-256 if not a git repo]`
   - `[TO FILL: snapshot date and destination path]`
4. The snapshot — not the volume — is the **baseline artifact**. All LOC
   counts, behavior comparisons, and data extractions reference the
   snapshot, so the baseline cannot drift while the rebuild proceeds.

---

## 1. Inventory (run once readable)

One pass, producing one committed document (`docs/design/throughline-inventory.md`)
that fills every blank below. Method notes are included so agents can do the
mechanical parts unattended.

### 1.1 Identity & stack

| Question | Answer | Method |
|---|---|---|
| Language(s) / framework(s) | `[TO FILL]` | file-extension census; lockfiles; build config |
| Build system & run command | `[TO FILL]` | README, Makefile/Taskfile, scripts |
| Does it currently run? | `[TO FILL]` | attempt launch from snapshot; record exact steps |
| Platform targets today | `[TO FILL: macOS only? web? both?]` | packaging config, conditional code |
| Last meaningful change | `[TO FILL]` | git log / file mtimes |

If the app does not run from the snapshot, **getting it running is part of
the inventory**, even if that means pinning old toolchains in a VM. A
non-running original cannot serve as the behavioral oracle (§1.7), and
without an oracle, "parity" is guesswork.

### 1.2 Feature list

Enumerate every user-visible feature. Method: walk every screen of the
running app; enumerate routes/menus/commands from source; read any README,
notes, or issue lists in the repo. Each feature gets an ID (`TL-01`,
`TL-02`, …) used throughout the parity matrix.

`[TO FILL: feature table — ID, name, one-line description, frequency of use
(daily / occasional / rare / never-actually-used)]`

The *frequency* column is load-bearing: it drives strangler ordering (§5)
and identifies waiver candidates (§4.1). Geoff fills it from memory of
actual use — this is the one column agents cannot fill.

### 1.3 Screens

Every screen, view, modal, and significant state. For each: a screenshot
from the running original (stored under
`docs/design/throughline-screens/`), the feature IDs it serves, and any
nontrivial interaction (drag, keyboard, live updates).

`[TO FILL: screen census — name, screenshot path, feature IDs, interactions]`

### 1.4 Data model

- Stores in use (SQLite? files? plist? other): `[TO FILL]`
- Schema dump (tables/entities, fields, relationships): `[TO FILL: method —
  e.g. `sqlite3 file.db .schema` if SQLite; otherwise per-format]`
- Total data volume and record counts per entity: `[TO FILL]`
- Export path: can the data be dumped to a neutral format losslessly?
  `[TO FILL]` — this becomes the input contract for the migration tool (§4.4).

### 1.5 Integrations & capabilities

- External APIs/services, with credentials and current liveness: `[TO FILL]`
- OS capabilities used (tray, notifications, file dialogs, global shortcuts,
  keychain, file associations…): `[TO FILL]` — each maps to the D5
  capability interface or exposes a gap in it.
- Background behavior (timers, sync, launch-at-login): `[TO FILL]`

### 1.6 LOC baseline

Measured on the snapshot, with the exact command recorded so the post-rebuild
count is comparable:

- Tool: `scc` (or `cloc`) with vendored and generated code excluded.
- Command: `[TO FILL: exact invocation + exclusion flags]`
- **Baseline total (app-specific LOC):** `[TO FILL]`

Definitions, fixed now so they cannot be gamed later:

- **Old app-specific LOC** = everything in the snapshot minus vendored
  dependencies and generated files.
- **New app-specific LOC** = everything in the rebuilt app's directory minus
  the Kit (imported, not copied — D13), minus `*_templ.go` and other
  committed generated code, minus vendored third-party. The Kit's own code
  does not count against the app — that is the entire thesis — but anything
  we write *in the app repo* counts, including its templ sources, `app.cue`,
  and config.
- Same tool, same flags, both sides.

### 1.7 Behavioral oracle

Keep the original runnable for the entire rebuild: pinned environment,
documented launch steps, read-only copy of its data. Every parity row in §3
is verified *against the running original*, not against memory.
`[TO FILL: oracle environment description]`

---

## 2. Mapping (each feature → the Kit, gaps → kit backlog)

For every feature ID from §1.2, one mapping row:

| Feature | Kit pattern | Kit components | Kit services | Status |
|---|---|---|---|---|
| TL-nn | e.g. list+detail w/ live updates ([ui-architecture-patterns.md](ui-architecture-patterns.md)) | e.g. table, detail pane, toolbar | e.g. `appkit/store`, `appkit/bus`, jobs | **have / partial / missing** |

`[TO FILL: complete mapping table after inventory]`

Rules of the mapping step:

- **Every `missing` becomes a kit backlog item** tagged `throughline-gap`,
  *before* the screen that needs it is built. The rebuild is the Kit's
  requirements probe; the tag is how we make the probe auditable (§6 walks
  the tag at the end).
- **Promotion rule** — where new code lives: build it in the app first
  unless it is *obviously* general (a second kit app would plausibly want it
  as-is). Promote app code into the Kit on the second use or on obvious
  generality, never speculatively. Speculative generalization is how kits
  rot into frameworks.
- **Mapping is also subtraction.** Features marked `never-actually-used` in
  §1.2 are waiver candidates (§4.1) — parity with dead weight is not parity,
  it is embalming. But the waiver decision is explicit and recorded, never
  silent.
- The Assistant should be exercised during mapping (D11/D15): for each
  feature, ask it "how does the Kit build this screen?" — wrong or empty
  answers are themselves `throughline-gap` items against the Assistant's
  pattern/guidance corpus, feeding north-star test #4.

---

## 3. Parity matrix (the rebuild's single pane of truth)

One row per feature ID, kept in this repo and updated as screens land. This
is the artifact Gate G4 reads.

| Feature | Old behavior (oracle ref) | New implementation | Evidence | Status |
|---|---|---|---|---|
| TL-nn | screenshot/recording + steps | screen/route in new app | test name, Gallery demo, or side-by-side recording | 🔴 / 🟡 / 🟢 / ⚪ waived |

Status definitions — fixed now:

- 🟢 **green**: behavior matches the oracle on web + macOS + Windows (where
  the feature applies per platform), with evidence committed (an automated
  test where feasible; a side-by-side recording where not).
- 🟡 **yellow**: works but with a known, recorded divergence pending
  adjudication (becomes green-with-note or red).
- 🔴 **red**: not yet built or divergent.
- ⚪ **waived**: deliberately dropped, with rationale recorded in §4.1's
  waiver log. Waived rows still count in the denominator of the parity
  report — we do not launder scope.

`[TO FILL: the matrix, seeded from §1.2 with all rows red]`

---

## 4. Acceptance criteria (Gate G4)

All five, together. Partial credit is not passing.

### 4.1 Feature parity

Every matrix row 🟢 or ⚪. The waiver log lives here:

`[TO FILL: waiver log — feature ID, rationale, date, Geoff's sign-off]`

### 4.2 Less app-specific code

New app-specific LOC ≤ **`[TO FILL: target %]`** of the baseline (§1.6),
same tool, same flags. Set the target immediately after inventory, before
the first screen is built — a target set late is a target negotiated.
Default ambition going in: **≤ 50%**. If inventory reveals the old app is
already unusually lean, adjust the target *with a written rationale*, not
silently.

### 4.3 Ships from one repo

Web + macOS + Windows from the rebuilt app's single repo, via the Phase 1
two-command build: signed, notarized (macOS), auto-updating through the
verified updater, browser-tab tier functional. No platform-forked source
trees; shells are chrome per the Loopback Invariant
([../adr/0005-loopback-invariant.md](../adr/0005-loopback-invariant.md)).

### 4.4 Data migration fidelity

- A **one-command, idempotent** migration tool (`cmd/migrate` in the app
  repo) reads the original's data (§1.4 export contract) and writes the new
  app's stores (D9 SQLite as durable truth).
- Verified by: entity counts match per type; field-level checksums or golden
  exports match for `[TO FILL: per-entity comparison strategy]`; spot-check
  protocol for non-mechanical fidelity (rendered output looks right, not
  just rows match): `[TO FILL]`.
- The original's data is preserved read-only throughout and after. The
  migration is re-runnable at any point during the strangler period (§5) —
  it will be run many times, by design.

### 4.5 Daily-drive test

Geoff uses the rebuilt Throughline exclusively for
`[TO FILL: duration — default 2 weeks]` with zero fall-backs to the
original. Any fall-back resets the clock and files the reason as a red
matrix row. Only after the clock runs out is the original archived
(snapshot retained permanently; oracle environment retired).

---

## 5. Method: strangler rebuild, screen by screen — not big-bang

**Decision: strangler-style rebuild in a fresh kit app, ported screen by
screen against the live oracle, with the data migration tool built first and
re-run continuously.** Not a literal strangler-fig (there is no shared proxy
layer routing between old and new — almost certainly meaningless for a
desktop app `[TO FILL: confirm once stack is known]`); the strangler
property we keep is *incremental displacement with the old system running
alongside as the reference*.

Why strangler wins here:

1. **The rebuild is a requirements probe, and probes need sample rate.**
   Each ported screen interrogates the Kit and emits `throughline-gap` items
   while the context is fresh. Big-bang delivers all of that feedback in one
   lump at the end — exactly when it is most expensive to act on and least
   attributable to a cause.
2. **Observable progress = honest progress.** Matrix rows go green one at a
   time. A big-bang rebuild is "90% done" for most of its life and nobody
   can falsify the claim. Gates-not-deadlines (the whole
   [../../ROADMAP.md](../../ROADMAP.md) posture) requires increments a gate
   can see.
3. **Agent-shaped work.** A screen — its oracle screenshot, its mapping row,
   its kit pattern — is a well-scoped, reviewable agent task. "Rebuild the
   whole app" is not. The developer+agent pair (D15) is the user of this
   protocol, and the protocol should hand that pair work it is good at.
4. **Risk containment.** A wrong kit abstraction discovered at screen 3
   costs three screens of rework. Discovered at screen N, it costs N.
5. **The daily-drive clock can start early.** Once the daily-use screens are
   green, §4.5 can begin while rare screens are still being ported —
   shrinking total wall-clock without shrinking rigor.

What big-bang offered, honestly: no dual-running period, no idempotent
importer requirement, and freedom to re-architect the data model wholesale
rather than maintaining migration compatibility per increment. We pay those
costs knowingly: the importer must be idempotent and re-runnable (§4.4
demands this anyway for verification), and the dual-running period is
exactly the oracle we need for §3 evidence. The data model *may* be
redesigned — the migration tool absorbs the delta; what we forgo is
deferring the migration design to the end.

**Build order:**

1. Walking-skeleton kit app (`throughline2` working name) via the Phase 1/5
   scaffold — running, themed, signed shell from day one.
2. Data model in the new shape + migration tool + fidelity checks (§4.4).
   Data first: every screen thereafter renders real, migrated data, so
   parity comparison is honest from the first screen.
3. Screens in order of **daily-use frequency** (§1.2), dependency-adjusted:
   the highest-frequency screen lands first so the daily-drive clock (§4.5)
   starts as early as possible.
4. Rare screens, integrations, waiver adjudications.
5. Final sweep: §6 donation audit, archive the original.

---

## 6. What Throughline donates back to the Kit

The rebuild is finished when the *app* passes Gate G4 — but the *Kit* is
only paid in full after the donation audit. Closing checklist:

- **Components**: any widget built for Throughline that a second app would
  plausibly use → promoted to the Kit, adapted to kit tokens, given a
  Gallery page (D12 — undocumented components fail the Gallery registry
  check). `[TO FILL: list]`
- **Patterns**: any screen shape not already in
  [ui-architecture-patterns.md](ui-architecture-patterns.md) → written up as
  a kit pattern with a Gallery demo, served by the Assistant. `[TO FILL]`
- **Services**: any infrastructure built app-side (sync glue, import/export,
  scheduling…) → evaluated against the promotion rule (§2). `[TO FILL]`
- **CUE schemas**: reusable manifest/config/token shapes discovered during
  the rebuild → into the Kit's schema library (D7). `[TO FILL]`
- **Migration tooling**: generalize `cmd/migrate`'s skeleton (read-legacy →
  validate → write-kit-stores → verify counts/checksums) into a kit data-
  import pattern — every future rebuild or adoption will want it. `[TO FILL]`
- **Test fixtures**: the oracle recordings and golden exports become the
  Kit's example corpus for visual-regression and migration testing where
  licensing of the content allows. `[TO FILL]`
- **Decisions**: every "why did old Throughline do X?" answered during the
  rebuild that turns out to be load-bearing → recorded, ADR-grade if it
  constrains the Kit ([../DECISIONS.md](../DECISIONS.md) discipline applies).
  `[TO FILL]`
- **The gap audit**: walk every `throughline-gap` item; each is either (a)
  built into the Kit, (b) consciously left app-local with a note, or (c)
  rejected with rationale. Zero unresolved tags at close.

---

## 7. Open blanks register

Consolidated list of every `[TO FILL]` above, so nothing silently stays
blank: baseline hash & snapshot path (§0); stack/run/platform/last-change
(§1.1); feature table with frequency (§1.2); screen census (§1.3); data
stores/schema/volume/export (§1.4); integrations & capabilities (§1.5); LOC
command + baseline number (§1.6); oracle environment (§1.7); mapping table
(§2); parity matrix (§3); waiver log (§4.1); LOC target % (§4.2);
per-entity comparison strategy + spot-check protocol (§4.4); daily-drive
duration (§4.5); proxy-layer confirmation (§5); donation lists (§6).

The protocol unblocks in one motion: grant the TCC permission, snapshot,
and run §1. Everything after that is execution.
