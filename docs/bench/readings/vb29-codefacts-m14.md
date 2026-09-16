## VB-29 (M14): code-fact classifier scored against a blind hand label

### Verdict

**The flag stays behind `--code-facts`.** Precision on the sentences the
classifier actually commits to (anchored-fresh, anchored-drifted or
unanchorable — i.e. excluding its own `preference` fallback) is **0.649**
(24/37) over the full 100-sentence corpus and **0.706** (24/34) on the
34 sentences all three labellers agreed on — both well under VB-28's
`>= 0.8` bar. The failure is not diffuse: `anchored-fresh` alone clears
the bar on its own (0.96, 24/25 correct) — every miss sits in
`anchored-drifted` (0/7 correct) and `unanchorable` (0/5 correct), the
two classes whose entire reason for existing is catching real drift and
declining unverifiable claims safely. On this corpus, every single time
the classifier called something drifted or unanchorable, it was wrong.
Root causes are identified below with citations; none of them is "the
detector needs more training data" — they are specific, fixable gaps in
`checkPathAnchor`'s two-candidate resolution and in how absence is read.

### Corpus

100 sentences, majority-labelled by three blind readers (Claude Sonnet 5
sessions with no access to each other's votes) reading real files at
commit `e1841718fa07` of this repository (plus two sibling files it
vendors and four Claude Code auto-memory files from three different
project memory directories): 29 distinct files across `CLAUDE.md`,
`AGENTS.md`, `internal/CLAUDE.md`, `docs/CLAUDE.md`, `cmd/fathom/CLAUDE.md`,
`schema/CLAUDE.md`, nine `.claude/skills/*/SKILL.md` files, the vendored
`third_party/agentic-kit/{CLAUDE,AGENTS}.md` and
`third_party/app-kit/{CLAUDE,AGENTS}.md`, four sibling clone `AGENTS.md`
files, and five Claude Code auto-memory notes. Harness split: 83
`claude-code`, 17 `codex-family` sentences (>= 20 files, two harnesses —
VB-29's own bar). Gold class distribution: 84 anchored-fresh, 11
anchored-drifted, 5 unanchorable. 85/100 rows are unanimous
(all three labellers agreed); the rest split 2-1.

### Labeller protocol

Three independent blind passes (no labeller saw another's vote or this
report) read each cited file at HEAD and answered "is this sentence a
code fact, and if so does it still hold": **anchored-fresh** (the
sentence's path/symbol/count anchor resolves and is still true),
**anchored-drifted** (it resolved once but the tree now disagrees — a
receipt names what changed), or **unanchorable** (code-fact-shaped but
names nothing checkable, or is a routing/preference instruction). The
recorded label is the majority of three; agreement rate is 85%.

### Method

`fathom config audit --code-facts --format json` was run against every
reachable root (this clone, its two vendored `third_party/*` trees taken
as their own roots so their own paths and symbols resolve, and the four
sibling scratch clones the corpus touches), then joined to the gold rows
by `(file, text)` with whitespace normalisation. **The join match rate
was 16/100** — far lower than expected — because `splitSentences` (the
file-to-sentence step, independent of the classifier under test) never
rejoins a sentence that a markdown hard-wrap breaks across two source
lines, which is the norm for this repo's prose; a fragment like
`"Surfaces: ... MCP server (stdio +"` / `"HTTP)."` never equals the full
sentence a human reader (or a labeller) sees. That gap, plus two
structural ones — `AutoLoadedFiles` never matches `SKILL.md` (only
`CLAUDE.md`/`AGENTS.md`), and Claude Code auto-memory resolves by
re-deriving `~/.claude/projects/<encoded-root>/memory` from the audited
`--repo`'s own absolute path, which can never equal the original
project's encoded path once the repo is read from a different checkout
— means the CLI-level join is not a usable scoring instrument on its own
here; **84 of the 100 gold sentences were "unmatched" by that literal
join** (misses for recall on the deployed pipeline, not something this
report charges against the classifier's own precision). Those 84 span
20 CLAUDE.md/AGENTS.md sentences whose sole issue is the line-wrap split
(their file is genuinely auto-loaded and reachable), 22 SKILL.md
sentences (never in the auto-loaded corpus at all), 13 auto-memory
sentences (never in the auto-loaded corpus from any of the sandboxed
roots), and the balance a mix of both. Of the CLI JSON's own findings in
gold-covered files, 1,112 carry no gold row at all — expected, since gold
sampled ~100 of the thousands of sentences those files contain ("ignored"
per the task's definition, not a quality signal).

Because that join could not carry the scoring load, **the classifier
itself — `classifySentence(root, sourceFile, text)` in
`internal/custodian/classify.go` — was called directly on each gold
sentence's exact text**, the same pattern this repository's own
`TestClassifySentenceLabelledSet` already uses to score the classifier
independent of file discovery and sentence splitting (its own doc
comment: "so the classifier's precision/recall can be measured directly
against a labelled set … independent of file discovery and sentence
splitting"). `root` and `sourceFile` were set to the real checkout the
sentence's own citations resolve against — `third_party/agentic-kit` and
`third_party/app-kit` as their own roots (their symbols and paths sit
outside a `vb29`-rooted walk, which skips `third_party/` by design), the
sibling scratch clones for the four rows sourced from them, and this
clone's root for the five auto-memory rows whose claims are about this
repository (`sourceFile` set to the real `~/.claude/projects/<encoded>/
memory/<file>.md` display form so `resolveRepoPath`'s own auto-memory
branch — no self-relative resolution — applies exactly as in
production). One gold row (`wave-n4/AGENTS.md`) could not be scored
against its original file: that scratch clone no longer exists in this
session's scratchpad; it was scored against `wave-n5`'s copy of the same
generated stanza line instead (same repository skeleton, same sentence).
This substitution is recorded on that row in the labelled set. This
harness was a temporary `_test.go` file inside `internal/custodian`
(same package, so it could call the unexported `classifySentence`
directly); it never entered the committed tree.

### Precision / recall / confusion (full corpus, n=100)

Exact 3-way match against the gold's own three classes (any classifier
call, including its own `preference` fallback, scored against the
majority label): **overall = 0.24** (24/100). That number is dominated by
the classifier's own precision-first design — a sentence needs a
concrete backtick-path, repo-relative slash-path, CamelCase symbol,
digit+unit count, or a narrow trigger phrase to be judged "shaped" at
all, so 63 of 100 real code-fact sentences (per the blind labellers, all
100 gold rows *are* code facts by construction — the labelling task
never offered a `preference` option) fell through to `preference`,
uncounted against the >= 0.8 bar by the design's own stated intent
("precision first … recall recorded, not floored"). The number the VB-28
bar is actually about is precision on the 37 sentences the classifier
committed to a real class:

| class | predicted | correct | precision | gold count | recall |
|---|---|---|---|---|---|
| anchored-fresh | 25 | 24 | **0.96** | 84 | 0.286 |
| anchored-drifted | 7 | 0 | **0.00** | 11 | 0.000 |
| unanchorable | 5 | 0 | **0.00** | 5 | 0.000 |
| **committed overall** | **37** | **24** | **0.649** | — | — |

Confusion matrix (gold row → classifier verdict; `preference` is the
classifier's fourth, non-gold class):

| gold ↓ / predicted → | anchored-fresh | anchored-drifted | unanchorable | preference |
|---|---|---|---|---|
| anchored-fresh (84) | 24 | 7 | 2 | 51 |
| anchored-drifted (11) | 1 | 0 | 3 | 7 |
| unanchorable (5) | 0 | 0 | 0 | 5 |

### Same numbers, unanimous subset (n=85, three-of-three agreement)

| class | predicted | correct | precision |
|---|---|---|---|
| anchored-fresh | 24 | 24 | **1.00** |
| anchored-drifted | 7 | 0 | **0.00** |
| unanchorable | 3 | 0 | **0.00** |
| **committed overall** | **34** | **24** | **0.706** |

Confusion (unanimous subset):

| gold ↓ / predicted → | anchored-fresh | anchored-drifted | unanchorable | preference |
|---|---|---|---|---|
| anchored-fresh (75) | 24 | 7 | 1 | 43 |
| anchored-drifted (6) | 0 | 0 | 2 | 4 |
| unanchorable (4) | 0 | 0 | 0 | 4 |

Restricting to unanimous rows *raises* precision (0.706 vs 0.649) —
every one of the classifier's correct calls survived into the unanimous
subset, and only its wrong calls got filtered out with the split-vote
rows. Anchored-fresh precision reaches a clean 1.00 (every predicted-fresh
row in the unanimous subset is actually fresh) but anchored-drifted and
unanchorable stay at zero: this is not a labelling-noise artifact, it is
the classifier's structural weakness on those two classes.

### Worst confusions, with examples

**anchored-fresh called `preference` (51 of 84 fresh sentences, the
largest single bucket).** The shaped-detector requires a clean
backtick-path, CamelCase symbol, digit+unit count, or one of five
trigger phrases; real prose routes around all of them with CLI flags,
descriptive text, or plain English claims about behavior:
- *"cp truncates a running executable, and an unsigned binary loses the
  daemon's TCC grant (deploy signs with the `fathom-dev` identity, then
  asserts the daemon serves HEAD)"* (CLAUDE.md) — `` `fathom-dev` `` is
  backticked but contains a hyphen, so it fails every backtick branch
  (not a repo path, not a bare file, not an identifier); the sentence's
  real claim (what the `deploy` Taskfile target does) has no anchor the
  detector looks for.
- *"`--url` defaults to `` `http://localhost:7575` `` (or `$FATHOM_HTTP`)
  — the daemon doctor queries read-only."* (fathom-doctor/SKILL.md) — the
  URL contains `://`, which repoPathRe's charset excludes, so the one
  backtick token that looks path-shaped never resolves to a check.
- *"Claude Code: 33 offered, 15 wired; Codex: 12 offered, 7 wired"*
  (fathom-doctor/SKILL.md) — four real counts, no unit word
  (`factNumberUnitRe` wants "N lines/files/tools/…"), no backtick, no
  trigger phrase — entirely invisible to the shaped detector.

**anchored-fresh called `anchored-drifted` (7 of 84 — every one of the
classifier's "drifted" calls that should have been "fresh").** All seven
trace to `checkPathAnchor`'s bare-filename resolution, which only tries
two candidates (repo root, and the *citing file's own directory* — built
for a nested `CLAUDE.md` describing itself):
- *"Root `CLAUDE.md` is a router (≤120 lines) with resolving pointers —
  `fathom config audit` (`.fathom/concerns.cue`) polices both."*
  (docs/CLAUDE.md) — the sentence says **Root** `CLAUDE.md`, but the
  bare-filename-prefers-own-directory rule ignores that qualifier and
  checks `docs/CLAUDE.md`'s own line count against 120, reporting drift
  on the wrong file.
- *"A `package.json` anywhere in **this repo** outside
  `apps/gallery/visreg/` is a build failure by policy."*
  (third_party/app-kit/CLAUDE.md) — the sentence's truth *is* that
  `package.json` does not exist; the detector treats every bare-filename
  mention as a positive existence claim and reports the correct absence
  as drift.
- *"`internal/tailwind/manifest.cue` pins the binary per platform from
  the release's official `sha256sums.txt`."* (third_party/app-kit/
  CLAUDE.md) — the first anchor (`manifest.cue`, a real repo path) holds;
  the second backtick, `sha256sums.txt`, is Tailwind's own upstream
  release asset name, not a local path, but it matches the bare-filename
  pattern and is checked as one — "any failure wins" then overrides the
  correct verdict from the first anchor.

**anchored-drifted called `unanchorable` or `preference` (10 of 11 — the
detector never once caught a real drift on this corpus).** Two distinct
causes: `../`-relative pointers are deliberately treated as foreign
(`resolveRepoPath` returns immediately when the first path segment is
`.` or `..`), which is exactly the shape of this corpus's most
on-the-nose drift example —
*"Built on **agentic-kit** (`` `../agentic-kit` ``, via `` `go.work` ``
+ `` `replace` ``)."* (CLAUDE.md) — go.mod uses an in-tree `replace`, not
a `../agentic-kit` sibling checkout, but the classifier can never see
that because the one path-shaped token it finds is punted as
out-of-scope by design. And the classifier has no notion of a build-time
or gitignored directory that is *supposed* to be absent in a fresh
checkout: *"installed copy at `` `~/.local/libexec/fathom/fathom` ``,
HTTP MCP `` `localhost:7575/rpc/agent` `` (16-tool surface), store
`` `workspace/db/fleet.sqlite` ``."* (platform-engineering auto-memory)
— `workspace/` does not exist at all in a fresh clone, so the top-level
segment pre-check in `resolveRepoPath` treats the whole path as foreign
rather than checking it, landing on `unanchorable` instead of the real
`anchored-drifted` (the tool count moved from 16 to 25).

### The discovery-layer gap (separate from the classifier's own precision)

Two structural findings surfaced by the join, worth recording even
though they didn't move today's number: (1) `AutoLoadedFiles` in
`internal/custodian/codefacts.go` never matches `SKILL.md`, so
`--code-facts` audits none of the nine shipped skills today even though
they are auto-loaded exactly like `CLAUDE.md`/`AGENTS.md` once a skill
fires; (2) `splitSentences` never rejoins a sentence a hard line-wrap
breaks in two, so most multi-clause prose in this repo's own
`CLAUDE.md`/`AGENTS.md` files is invisible to the classifier as a whole
sentence even where the file itself is correctly discovered. Neither
finding blocks or unblocks the flag on its own; both are follow-up
material, not scored here.

### Verdict against VB-28's bar

VB-28 shipped `--code-facts` "behind a flag until precision >= 0.8 on
twenty real files." This run used >= 20 real files across two harnesses
(29 files, 83/17 split) and measured **0.649 committed precision**
(0.706 on the unanimous subset) — under the bar on both cuts, and the
failing classes are exactly the two the flag exists to police
(`anchored-drifted`: 0/7; `unanchorable`: 0/5). **The flag stays behind
`--code-facts`.** The fix is not "more data": every miss above traces to
a named, specific gap in `checkPathAnchor` (bare-filename resolution
scoped to two directories, no absence-is-correct handling for
gitignored/build paths, no repo-relative resolution into a vendored
`third_party/` subtree, `../`-relative pointers always foreign) that a
follow-up row can address directly against these eleven examples.
