# Collision-free append points: change fragments, regenerated boards, sibling plan files, a derived epoch

Status: proposed 2026-10-19 (wave f88d). Promotes F88 (`docs/intent/futures/f86-f91.md`) by its own trigger.
Plan: `docs/plans/append-points.md` (AP-). Evidence: the F82 replay
(`docs/bench/readings/f82-overlap-replay-2026-10-19.md`, 523 merges, 263 conflicted, 247 on an append point) and the
hunk census in section 2 (git only, no model, reproducible by section 8). Builds nothing: every decision below is a
plan row.

## 1. Two horizons (intent-design)

- **F88 Promote when fired**: 247 append-point merges in 30 days against a bar of 4. Drop if (< 1) is far away.
- **F88 Do not build: a lock or a reservation.** Honored: no lock, no slot reserved in a file. The per-brief id
  reservations already in use stay (section 7); they partition counters, they reserve no line.
- **F82** (line overlap) fired on the same reading. Its detector is not built here; this design removes the
  append-point class so what is left is the overlap F82 is about.
- **F73** (per-extractor epoch salts): Do not build before IXS-M2 reads under 0.5; Preserve `FingerprintHash` as the
  one hash-gate formula. D4 keeps one global salt and `FingerprintHash`, so F73 stays open and unbuilt.
- **ADR-074**: a committed view needs a named reader and a check, or it is not committed. D1 commits no
  `[Unreleased]` view. D2 keeps only views that have both (the milestone line has LB-M6 as reader).
- **ADR-048 decision D2** (one ExtractorEpoch bump per wave, made by the integrator): D4 replaces the mechanism, so
  the build amends it.
- **Kit `changes.md`** sections 4, 6, 8 and 11. Fragments are neither a generated changelog nor a second log (the two
  section 11 rejections): the agent still writes the line in the same change; only where it waits until the roll
  changes. The kit amends its practice first, fathom adopts.
- Constraints carried in: ADR-066 (pure Go), ADR-011 (no org names), I54 and I55 unchanged, the table freeze (no
  store table, cache or job; D4's digest is computed once per process from embedded bytes and never stored).

## 2. What conflicts, measured

Hunk census: every replay merge listing the file, re-merged with `git merge-tree`, each conflict hunk classed by its
lines.

| File | Merges | Cause | Design |
|---|---|---|---|
| `CHANGELOG.md` | 118 | every change adds lines under the same `[Unreleased]` category headings and edits the one `Release impact:` line | D1 |
| `docs/plans/language-breadth.md` | 64 | 49: both sides insert a new `### F.NN` foundation section (or rows) at one point, the end of section F (every hunk add-only, empty base), often with the same F number; 13: both sides edit the same or touching queue rows (a blank line between rows clears none of the 64 merges); 2 other | D3 |
| `docs/languages.md` | 56 | 139 hunks: the milestone line (38 merges), the two `**Total**` rows (28), tier count rows (27), table rows (34: support table 21, board lists 12, the template's typed-edge table 8). 21 merges conflict only on derived summary lines. The whole file is generated (`TestLanguagesTrackerIsGenerated`) | D2 |
| `.fathom/observations.md` | 46 | last on 2026-10-01. No writer remains: `fathom observe` writes one file per entry under `.fathom/observations/` (0 conflicts in the window) and `cmd/fathom/observations.go` reads the legacy log and never writes it | none |
| `internal/fusion/extractor_epoch.go` | 17 | every hunk is the file's end: the new history paragraph and the `const` both sides bump. Every side of all 17 also changed extraction Go (1 to 24 files). `internal/fusion/pipeline.go` adds 8 more before the 2026-09-30 split, same cause | D4 |
| `internal/extractor/tsengine/grammars/PROVENANCE.md` | 14 | classed OTHER by the replay, so outside the 247. 12 conflict on the `**total**` row, 10 only there; 4 on adjacent rows of the sorted tables; 2 also on `grammars.lock.json` | D2 |

## 3. D1: change-note fragments

**Layout.** Each change with an observable effect adds one file under `docs/changelog/unreleased/`, named for its
line of work (`<branch>[-<topic>].md`, unique because branch names are). The file holds exactly one Keep a Changelog
heading (`### Breaking`, `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed`, `### Security`)
and one or more `- ` lines, each held to the kit's line rule (at most two sentences, a link to the why). A change in
two categories writes two files. Collapse while unreleased (kit section 4) still holds: a fix to an unreleased
addition edits that fragment, and a removal deletes it.

**`CHANGELOG.md`.** `## [Unreleased]` keeps one static line, `Unreleased changes wait as fragments under
docs/changelog/unreleased/; the release ritual folds them here.` Nothing edits it between releases. No `Release
impact:` line is kept: the roll computes it.

**The roll.** The kit's `release-notes` command gains the fold. `release-notes -roll x.y.z -date YYYY-MM-DD` reads
every fragment in bytewise filename order, writes `## [x.y.z] -- date` under the pointer with `Release impact:` (the
highest category; pre-1.0, Breaking bumps minor), then the categories in the fixed order above, `### Breaking`
first, each category's lines in fragment order then file order, and deletes the fragments in the same commit. The
date and version are flags (no clock). `release-notes Unreleased` prints the same fold to stdout and writes nothing:
the on-demand view for the owner catching up (kit section 7). `release-notes -check` lints the fragments.

**Readers that must see fragments.** `fathom plans archive` absorption (`readAbsorptionFiles` globs
`docs/changelog/*.md`, one level only), the custodian ADR-reference concern (`adrCitingFiles` names
`CHANGELOG.md`), `intent-doctor` rows 11 and 12, `intent-prune` pass 5, `intent-release` steps 1 to 5, the
`AGENTS.md` change-records stanza, `CHANGELOG.md`'s header and the router's "what changed" row. `kit-new` writes into
`[Unreleased] > Added`; in a fragments repo it writes a fragment (kit change).

**Check.** A gate test fails when a fragment has no heading, two headings, an unknown heading or no line; when
`[Unreleased]` holds anything but the pointer (an agent's habit of appending there); and when a fragment line also
appears in a released block (a roll that folded and did not delete).

**Removes** 118 merges' `CHANGELOG.md` conflicts. **Cost**: kit practice text, the fold and check in
`release-notes` (about 200 lines of Go and tests), three skill edits; fathom: the directory, a one-time move of
today's `[Unreleased]` lines into fragments, two readers, the gate test, four doc lines. **Risk**: in-flight branches
that still append to `[Unreleased]` conflict once with the pointer (move their line to a fragment; do the move between
waves); an edit to another line's fragment can still conflict, which is a real overlap. **Kit vs fathom**: the
practice, fold, check and skills are kit (`changes.md` sections 4, 6, 8, 11); fathom adopts with its path.

## 4. D2: a generated file merges by regeneration

**Remove the cause where the line has no reader.** PROVENANCE.md's `**total**` row has no reader outside its own
render (grep of plans, tests and docs): drop it from `RenderProvenanceTables`. That alone clears 10 of 14 merges, in
every clone, with no configuration. The languages board keeps its totals and milestone line: the board is their
reader and LB-M6-M1..M3 read the milestone line lexically.

**Resolve the rest by the generator.** `.gitattributes` names each committed generated file with
`merge=fathom-regen` (`docs/languages.md`, `internal/extractor/tsengine/grammars/PROVENANCE.md`). The driver keeps
the current side and reports success (`git config merge.fathom-regen.driver true`, set by `task merge-drivers`; git
config is per clone, so `scripts/cloud-setup.sh` and the wave clone step in `docs/guides/agent-waves.md` run it).
After any merge the integrator runs `task regen`, which runs every generator in a fixed order (`indexbench
lang-tracker`, the provenance render) and commits what changed. A clone without the driver gets today's text merge
(verified: an undefined driver conflicts, `true` merges clean keeping the current side). A forgotten `task regen`
cannot pass: `TestLanguagesTrackerIsGenerated` and `TestProvenanceRenderedFromLock` compare the file with its render
at the gate.

**Removes** all 56 `docs/languages.md` merges (the 6 that also conflict on `internal/langbench/languages.tmpl.md`, an
input, keep that conflict, which is a real template edit) and 12 of 14 PROVENANCE.md merges (2 keep a
`grammars.lock.json` conflict). **Cost**: two attribute lines, two tasks, one renderer line, three setup lines.
**Risk**: a merge commit can hold a stale generated file until `task regen` runs; the gate catches it. **Kit vs
fathom**: kit `operating.md` section 4 (parallel agents) gains the rule "a generated file merges by regeneration,
never by hand"; fathom adopts with its files.

## 5. D3: the language waves' foundation sections in sibling plan files

New foundation work of a wave goes in its own plan, `docs/plans/language-breadth.<wave>.md`, beside the existing
`language-breadth.queue.json`. Existing sections F.1 to F.73 stay where they are (moving them would conflict with
every wave in flight). The sibling is an ordinary plan: grammar v2 and `fathom verify` are unchanged, the gate walks
it because it is top-level, and `fathom plans archive` moves it out once its rows are proven, so gate cost stays
O(active plans). LB-F ids stay unique by the per-brief reservation; the F.NN section number goes (a file per wave
needs no counter). A queue entry that waits on a new row uses the cross-plan form
`language-breadth.<wave>.md#LB-F<n>` (`internal/verify/crossplan.go`). One reader changes: `support.PlanRowMarks`
reads the siblings as well as `language-breadth.md` for the support console. The queue section Q and the measurements
stay in the main plan (`TestLanguageQueueMatchesPlan`).

**Removes** 49 of 64 merges. The 13 queue-row edits stay: they are F82's overlaps, not append points. **Cost**:
one reader, the worker recipe (design `language-breadth.md` section 13) and the plan header. **Risk**: more plan
files until archived; a needs edge written in the old single-file form fails verify loudly. **Kit vs fathom**: the
kit rule in D2's paragraph covers it ("a parallel line adds a file, never appends to a shared one"); the layout is
fathom's.

## 6. D4: ExtractorEpoch from bump files and a data digest

**Evaluated as briefed: a content hash of the embedded dialect manifests and grammars as the data part, a manual code
component kept.** The data part alone removes 0 of the 17 conflicts, because every conflicting side also changed
extraction Go and so still bumped the manual part. Its value is correctness: today a dialect manifest, query or
grammar change shipped without a bump leaves stored observations stale (the O9 class); with the digest it cannot.
The conflict goes away only when the manual part stops being a shared line.

**Design.** The manual part becomes one file per bump, `internal/fusion/epochs/<wave>.md`, holding that bump's history
paragraph (what changed, why stored observations are stale), embedded with `go:embed`. Today's history comment
becomes the frozen base, 67. `ExtractorEpoch` becomes a string var computed once: `<67 + number of bump
files>.<first 12 hex of sha256(sorted bump file names and bytes, the data digest)>`. The data digest is sha256 over
the sorted paths and bytes of `internal/extractor/dialects/*/*.json` and `*.scm` and `grammars.lock.json` (the lock
pins every blob's sha256, so no blob is hashed at start). `FingerprintHash` keeps its formula and salts with the new
value.

**Re-ingest semantics.** Before: a manual bump changes the salt; the next ingest of every service re-extracts every
file. A data change without a bump re-extracts nothing. Two waves that both bump conflict, and the integrator
resolves to the next number. After: a new bump file or any embedded data byte changes the salt, and the next ingest
of every service re-extracts every file, the same breadth as a bump (F73's per-dialect salts are not built). A merge
of two waves yields a value distinct from both, so one re-extract follows integration with no hand resolution. The
first ingest after the upgrade re-extracts once because the value's form changes.

**Removes** 17 merges (and the 8 pre-split ones in `pipeline.go` cannot recur). **Cost**: about 80 lines of Go and
tests, the ADR-048 D2 amendment, the worker recipe and the waves guide. The 15 per-wave epoch tests keep passing
(each seeds a prior literal epoch); new ones assert that the value with the wave's bump file differs from the value
without it. **Risk**: a non-semantic data edit (a `display` string) now costs a full re-extract (65 bumps landed in
30 days, so the added rate is small); `extractor_epoch` in `fathom version --json`, the SARIF run properties and
verify provenance stays a string but changes form. It is not on the compatibility surface (`docs/intent/scope.md`),
so it is a Changed line. **Kit vs fathom**: fathom only.

## 7. Ids

Keep per-brief reservations for O, ADR, F, LB-F and plan-row ids. Nothing cheaper holds up: content-derived ids are
unreadable in prose and commits; wave-scoped ids (`O-f88d-1`) break every `O\d+` and `ADR-\d{3}` reader; renumbering
at merge rewrites every reference. One-file-per-entry already removed the observation append point (0 conflicts in
`.fathom/observations/`). The `ADR.md` (5) and decisions index (4) conflicts are appends at the end of the file, not
id collisions; splitting `ADR.md` per record is not worth its line-pinned fixtures at that rate. `fathom observe`
picks the next free id and has no way to take a reserved one, so workers write reserved observations by hand (O1343).

## 8. Projection and measurement

Applied to the replay table, D1 to D4 clear about 169 of 247 append-point merges, leaving about 78 (32%). The rest:
plan edits (51 merges conflict on nothing but plans, 12 of them on `language-breadth.md`, whose 15 edit merges all
remain; mostly the same rows edited on both sides, counted as append points by the replay's path rule, O1342),
`docs/mcp-tools.md` 7, `internal/connector/languages_test.go` 6, `ADR.md` 5, the decisions index 4,
`internal/connector/typededges.go` 4, `internal/connector/connector.go` 3. The briefed
target, at most 25% of 247 (61), is therefore missed by about 17 merges unless plan same-row overlaps fall as well.
AP-M1 reads it as it lands. A blank line between plan rows was simulated over all 105 plan-conflict merges: it
cleared 13, so it is deferred, not adopted.

AP-M1 re-runs `docs/bench/readings/f82-overlap-replay.py` over the 30 days after the last build row, with the same
classifier, from a clone where `task merge-drivers` has run (without it the replay counts conflicts that integrators
never saw). It reports append-point merges per month and per 100 merges (today 247 of 523).

Census commands: for each merge `m` and file `f` in the replay table, `git merge-tree --write-tree m^1 m^2`, then
`git show <tree>:f`, read the hunks between the conflict markers. The blank-line simulation applies the same three-way
merge (`git merge-file`) to base and both sides with a blank line after every row.

## 9. Considered and rejected

- `merge=union` for `CHANGELOG.md` or plans: it keeps both sides of every conflicting hunk, edits included, so a
  collapsed line or an edited row survives twice. Wins if: a file is append-only by rule and a check catches a
  duplicate.
- A committed `[Unreleased]` view rendered from fragments: no reader the on-demand render does not serve, and it
  brings the append point back. Wins if: a consumer must read the unreleased notes from the file at HEAD; then it is
  generated, checked and merged by regeneration (D2).
- A lock or a per-file slot for append points: F88 and F82 reject them for now.
- A blank line between plan rows: 13 of 105 plan-conflict merges cleared in simulation. Wins if: adjacent-row plan
  conflicts reach 10 a month after D3.
- `ADR.md` split into one file per record: wins if `ADR.md` append conflicts reach 8 a month.
- Per-dialect epoch salts: F73, unbuilt by its own condition.
- Per-dialect registration in place of the connector's shared per-language maps (F88's motivation;
  `typedEdgeProducers`, `extClassMap`, `langRowHint`): their three files conflict in 7 of the 247 merges. Wins if:
  they conflict in 8 merges a month (AP-D4).

## Decision models

None applies (ADR-073): every decision here is a deterministic file layout or a pure, sorted fold, so no typed
question exists whose answer a model could improve, and provider `none` is the whole design.
