Verdict: no recompile needed

# Does splitting ADR.md into per-file records force a paid adr-rules recompile?

Measured 2026-10-17 at `9588c4e5`/`d035164a`. No model spend, no compile run: the section arithmetic ran offline, over `internal/semantic.SplitMarkdownSections` and the repository's own `ADR.md`.

## Answer

No, for a pure move: each `## ADR-NNN` chunk written to its own file as a byte-identical copy of its lines. The compile engine already carries a section's claims across a move (AG-26, `internal/semantic/carry.go`) with no model call, and every one of ADR.md's 76 sections meets the carry's digest rules. A split that edits a body, adds a title or front matter, or re-levels a heading changes that section's digest and that section pays.

## How adr-rules rules are keyed

| Key | Where | Effect of a pure move |
|---|---|---|
| Subject `<path>#<slug>` | `SplitMarkdownSections`, `sections.go` (slug of the heading, `-2` dedupe per file) | Changes: the path is part of it. A new subject has no claims, so the ordinary digest gate would compile it. |
| Input digest, sha256 of the section text (heading line through the line before the next heading, document coordinates not included) | `Section.Digest`; `allDigestsMatch` (`compile.go`) | Unchanged when the lines are byte-identical. |
| Prompt identity `adr-rules/<8 hex of prompt.md>` | `promptForLayer` (`prompts.go`): the prompt text only | Unchanged. `layer.cue` (including `corpus: include`) is not part of it. |
| Citation `path#Lstart-Lend` | claim links, `RelCites` | Changes (new path, new lines); carry re-anchors it by one line shift (rule 5). |
| Line range | `Section.StartLine/EndLine` | Never a key; recomputed from the file. |

So subject and citation re-key; digest and prompt identity do not. AG-26 exists for exactly that split: a section whose digest and prompt identity match a section compiled at another path keeps its claims, re-cited, no model call.

## The carry rules against ADR.md

1. Own subject has no claims at the digest: true for every section of a new path.
2. Exactly one departed subject holds that digest: true while the old `ADR.md#...` claims are still in the store (the supersede sweep retires them after the carry).
3. Exactly one current section has the digest: measured, 76 of 76 digests unique in `ADR.md` and in the split (0 duplicates either side).
4. Same scope floor, neither a self-scoping rules file: `scopeFloorDir` is `""` for `ADR.md` and for any `docs/...` file that is not an instruction file; `declaredDocScope` does not apply to plain markdown.
5. Every cited range re-anchors by one line shift onto hash-matching lines: measured against the stored claims below (Measured 2026-10-17), 0 failures in either layer. A section that fails it compiles as before.

Offline measurement (scratch program, not committed): `ADR.md` is 546,872 bytes, 76 sections (the preamble plus 74 `## ADR-NNN` records plus one more heading). Cut at the 74 `## ADR-NNN` headings outside code fences, with the preamble as its own file: 76 sections, all 76 original digests present in the split, 0 missing, 0 duplicated. The file content is the section text verbatim (the text's trailing blank line is the file's final newline), which is what keeps the digest.

## What the split would cost

Model spend: 0 for every section that meets rule 5 and whose current claims are fresh. A section with stale claims today pays on the next compile whether or not the file moves; the split does not cause it. Ceiling if every section failed rule 5: both layers that include `ADR.md` (`obligations` and `adr-rules`) carry `max_usd_per_run: 1.0` and `max_tokens_per_run: 200000`, so one run per layer, $2.00 at most, and the budget gate stops it there. Expected: near zero.

Required, none of it spend:

- `corpus: include` in `internal/semantic/layers/adr-rules/layer.cue` and `internal/semantic/layers/obligations/layer.cue` gains the new directory (and drops `ADR.md`). Pack data, not prompt identity.
- A `fathom compile --layer adr-rules` and `--layer obligations` after the split, to run the carry pass. Until it runs, the old claims' source file is gone: since IV-7 `obligations` withholds those rules and counts them (`layer_coverage.missing_sources`, remedy names the recompile) and the raw fallback serves the current section text, so nothing stale governs in the gap. Run the compile in the same change.
- `ADR.md` is also read by 25 non-test Go files and named in 66 test files (`grep -rl 'ADR\.md' --include=*.go`); the readers that open the real file are `internal/adr` (Parse and the single-file `Sections`), `internal/custodian/adrrefs.go`, `internal/intent` (corpus, triggers, bets, visions), `internal/semantic/h1governance_test.go` and cmd/fathom's `decisionmodels`, `orgnames`, `edithook_budget`, `intentaudit` tests. The split is a code change across those, not a data move.

## Fixtures that pin an ADR.md line number

Line pins of the form `ADR.md#L<n>` or `ADR.md:<n>` (`grep -rlE 'ADR\.md(#L|:)[0-9]+'`):

- Three JSON fixtures: `testdata/contextbench/tasks-home.json` (14 occurrences), `testdata/scenariobench/guards-adr.json` (9), `testdata/scenariobench/lookups-vs.json` (6, mostly historical prose). 29 occurrences.
- Two are verified against the live file and are the ones that broke on 2026-10-17 when a five-line insertion went in under ADR-018: `tasks-home.json` H1-T07 and `guards-adr.json` guards-adr-014 (fixed in `d035164a`, `ADR.md#L723` to `L728`, `L576` to `L581`). H1-T08 and H1-T12 also pin lines in `ADR.md`/`CLAUDE.md` (the file's own maintenance note says an edit above the cited line fails the test).
- Go sources with a line-citation string: `internal/indexbench/claims.go` (`ADR.md#L40-L42`, a seeded store row, not read from the file), `internal/scenariobench/citation.go` and `internal/contextbench/task.go` (comments). The other 19 Go test files that match are fake `Src` strings in unit tests (`ADR.md#L650-L651` and the like) that never open the real file.
- `docs/bench/scenarios-ledger.jsonl` (13) and `docs/bench/scenario-runs.md` hold historical readings; they are records, not pins.

Under a split every one of the live pins changes meaning: the cited file becomes `docs/adr/<record>.md` and the line becomes file-relative, so each of the 2 verified pins (and H1-T08, H1-T12) is re-derived once, and from then on an insertion in one record moves only that record's lines. That is the case for splitting, not against it: today a five-line insertion under ADR-018 shifts every pin below it.

## Consequence for ADR-074

ADR-074's Revisit when clause "IV-9 finds a split needs no paid recompile" has fired on this reading: the split is its own change, gated by `task validate` and a free `fathom compile` carry pass, not by spend approval. Preconditions to state in that change: byte-identical bodies, `layer.cue` corpus edits in both layers, the carry compile run in the same change and its `SectionsCarried`/`SectionsCompiled` counts recorded, and the rule 5 failures (if any) named with their cost before any model call.

## Measured 2026-10-17

Rule 5, measured at `a7d18c2f` against the stored claims. No model spend, no compile run, the shared store untouched: the daemon's store (`workspace/db/fleet.sqlite`, the `--store` of the running `serve-http`) was copied with sqlite3 `.backup` to a scratch file on the external volume (837 MB), and a scratch test (not committed) ran the carry's own path, `planCarries` (rules 1 to 4) and `carrySection`/`citationShift` (rule 5), over the copy's live claims and a simulated split of `ADR.md`: the preamble plus one file per `## ADR-NNN` record, 75 files, 76 sections. The stored claims come from the last `adr-rules` compile, 2026-09-25 at `fa8c8ed8` (Haiku 4.5, 344 sections, $0.690213), under `adr-rules/3343fdef`, which is still the prompt identity.

| Layer | Live ADR.md claims (subjects) | Re-anchored (carried) | Would recompile, caused by the split | Stale today, pays anyway |
|---|---|---|---|---|
| `adr-rules` | 78 (21) | 34 claims, 10 subjects, 112 links | 0 | 44 claims, 11 subjects (78 KB) |
| `obligations` | 448 (61) | 350 claims, 53 subjects, 1,084 links | 0 | 98 claims, 8 subjects (106 KB) |

Every claim whose section is unchanged since its compile re-anchors: 0 rule 5 failures in either layer. The 44 `adr-rules` claims that do not carry sit in sections edited after 2026-09-25, so their digest already differs and they recompile on the next compile, split or not. The split itself costs $0.

The precondition is how each file ends. A section's text runs to the line before the next heading, so the blank line between two records belongs to the earlier section only as a line, not as a final newline. The files must hold each record's lines with that blank separator line dropped (the file then ends in one newline; rejoined with `\n` they give `ADR.md` byte for byte): 76 of 76 digests kept, all of the above. A naive cut whose files concatenate to `ADR.md` exactly keeps the separator line, so 74 of the 75 files end in `\n\n`, each of those sections' text gains one newline, and 74 of 76 digests change. The carry then matches nothing (rules 1 and 2; `citationShift` alone would still find a shift for every section), and 10 `adr-rules` subjects (34 claims, 48 KB) and 53 `obligations` subjects (350 claims, 259 KB) recompile.

Dollar ceiling for those recompiles: each layer's `max_usd_per_run: 1.0` stops a run at $1.00, so $1.00 for `adr-rules` and $2.00 for both layers. Estimated at Haiku 4.5 list price ($1 in and $5 out per million tokens; input as prompt plus section bytes over 4; output at the 2026-09-25 run's 0.096 output-to-input ratio): about $0.03 for `adr-rules` and $0.13 for `obligations` under the naive cut, $0 under the correct cut. The stale sections cost about $0.04 and $0.04 on the next compile either way.

Separate from the split: 55 of the 76 sections hold no `adr-rules` claims (most are records written after the 2026-09-25 compile, about 420 KB) and 15 hold no `obligations` claims (182 KB). The digest gate compiles a section with no claims on every run (`needsCompile`: no prior claims means compile), so they pay on the next compile whether or not the file moves (about $0.20 and $0.08). The split neither causes nor saves that cost.

For the split change: cut each record without its trailing blank line, check that 76 of 76 section digests survive (`SplitMarkdownSections` over old and new), run `fathom compile --layer adr-rules` and `--layer obligations`, and record `SectionsCarried` (10 and 53 if nothing moves first) beside `SectionsCompiled`.
