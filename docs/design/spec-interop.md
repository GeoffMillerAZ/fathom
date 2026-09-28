# Spec interop: foreign spec dialects verified in place, and the certified tier

Status: accepted 2026-09-27 (ADR-049 decisions 6 and 7). Vision:
`docs/vision/38-proof-layer-roadmap.md`. Plan: `docs/plans/spec-interop.md` (SI-). Goal G19.
Promotes futures F20. Constraints carried in: plan grammar v2 (`docs/plans/README.md`, the
compatibility surface), ADR-043 (`certified_by:`, liveness), ADR-044 (`tests:` is a `gone:
callers` modifier), ADR-042 (the decision port and its calibration rule), law 15 (write only
inside fathom's markers), the "no separate SDD binary" and "no tracker" non-goals.

## 1. Problem

Spec-driven development went mainstream in 2026: Spec Kit, OpenSpec, Kiro, BMAD, Task Master
and Superpowers turn goals into specs and task checklists. Their "is it done" step is a
model's opinion (Spec Kit's converge, OpenSpec's verify, which does not even block
archiving). Fathom's gate reads code, but only in its own grammar. People will not rewrite
their specs into ours; the proof has to come to their files.

## 2. Decisions

1. A spec-dialect port. Plan grammar v2 is the canonical model; every dialect adapter maps a
   foreign file into `verify.Plan` and back to positions in the foreign file.
2. Read in place, never rewrite. Fix-up tasks are written only when asked, only inside a
   `<!-- fathom:begin converge -->` ... `<!-- fathom:end converge -->` block (law 15).
3. The converge report is the product: claimed-unproven tasks, tasks with no anchor, and code
   that nothing claims.
4. Our grammar gains two additive keys, `weight:` and `accept:`, and one header,
   `Proof: certified`. The test-tied tier is `certified_by:` (ADR-043). `tests:` is not
   reused: it already means a `gone: callers` modifier (ADR-044, compatibility surface).
5. Jev proposes anchors for unanchored foreign tasks, behind a calibration gate. A proposal
   is never proof.

## 3. The dialect port

```go
type SpecDialect interface {
    ID() string                         // "fathom-v2", "speckit", "openspec", "kiro", "bmad", "taskmaster", "gh-tasklist"
    Detect(root string, files []string) []string   // candidate spec files, deterministic
    Parse(path string, content []byte) (verify.Plan, []Span, error)
}
type Span struct{ TaskID string; Line, Col int } // where a task's checkbox sits in the foreign file
```

Detection is by path and content shape, never by a model. `fathom verify --plan <path>`
dispatches to the dialect that claims the path (law 10: an existing verb, no new tool);
`fathom verify --dialect <id>` overrides. Output keeps the exit codes 0/1/2 and reports lines
in the foreign file.

| Dialect | Files | Task unit | Done marker | Anchor sources | Wave |
|---|---|---|---|---|---|
| fathom-v2 | `docs/plans/*.md` | checkbox row | `[x]` | `{symbol, oracle, gone, certified_by}` | exists |
| Spec Kit | `specs/<feature>/tasks.md`, `spec.md`, `plan.md` | `- [ ] T001 ...` | `[x]`/`[X]` | file paths named in the task text; `[P]` parallel markers ignored | 1 |
| OpenSpec | `openspec/changes/<id>/tasks.md`, `specs/**/spec.md` | numbered checkbox | `[x]` | paths and symbols named in the task; requirement ids from `spec.md` deltas | 1 |
| Kiro | `.kiro/specs/<name>/requirements.md`, `design.md`, `tasks.md` | checkbox with `_Requirements: 1.2_` | `[x]` | requirement ids (EARS) and file references | 4 |
| BMAD | story files under the configured stories directory | story with acceptance criteria and a task list | status field plus checkboxes | file list section | 4 |
| Task Master | `.taskmaster/tasks/tasks.json` | task and subtask objects | `status: done` | `details` and `testStrategy` text | 4 |
| GitHub task lists | issue or PR bodies exported to a file the user passes | checkbox | `[x]` | paths and symbols in the text | 4 |

A foreign task with no extractable anchor verifies `no-contract` (Unknown) exactly as a
native row without an annotation does; nothing is guessed into a contract.

### 3.1 Inline anchors in foreign files

An author may add a fathom annotation to a foreign task line (`{symbol: CreateOrder,
package: internal/orders}`); every adapter reads it with the plan grammar's own tokenizer, so
there is one quoting rule. This is the upgrade path from "prose task" to "provable task"
without leaving the author's tool.

## 4. Grammar additions (additive, compatibility surface when they land)

| Key | Meaning | Default | Gating |
|---|---|---|---|
| `weight:` | positive integer, the row's share of a weighted rollup | 1 | never; read by progress views (`docs/design/progress-and-economics.md`) |
| `accept:` | acceptance text, EARS-shaped where the author likes ("WHEN <trigger> THE SYSTEM SHALL <response>"), quoted | unset | never by itself; shown in the converge report and the traceability matrix; proven only through `certified_by:` |
| `Proof: certified` (plan header line, like `Baseline:`) | every `symbol:` row in the plan must also carry `certified_by:`; a row without it is refused `certificate-absent` | unset (today's tiers) | yes, through the existing `EvaluateCertifiedBy` |

`accept:` joins `annotationKeys` so a mangled block stays loud (I22). Lint refuses `weight:`
below 1 or non-integer.

**The certified tier.** A foreign requirement-to-test reference (Kiro's
`_Requirements:_` plus a test named in the task, a Spec Kit task that names a test file, a
Task Master `testStrategy` naming a test) maps to `certified_by:` on the synthesized row. The
row verifies only when that test's certificate is `live` (the newest attested run passed at a
clean commit and the reach is unchanged, ADR-043). "Done requires a passing linked test" is
therefore already the meaning of `certified_by:`; this design only makes it reachable from
foreign dialects and a plan default.

## 5. The converge report

`fathom verify --converge` (a flag on the existing verb) prints, per spec:

1. **Claimed-unproven:** tasks marked done that fail their contract, with the failing
   criterion.
2. **Unanchored:** tasks with no contract, each with up to three anchor proposals (§6) marked
   as proposals.
3. **Unclaimed code:** symbols added or changed on the spec's branch range
   (`git diff base...head`, bounded by the changed-file set) that no task anchors. The base is
   the spec's own creation commit, or `--base`.

With `--write-fixups` and only then, each claimed-unproven and unclaimed item becomes a task
appended inside the fathom converge block of the foreign `tasks.md`, in that dialect's own
checkbox shape, so the author's tool shows it. Re-running replaces the block's contents; text
outside the markers is never touched.

## 6. Jev: anchor inference, calibrated or absent

- **Where it helps:** an unanchored foreign task ("Add retry to the payment client") has a
  small candidate set from `find_symbol` and `pack_context`. A Choice over at most eight
  candidates is exactly the decision port's shape (ADR-042) and the strongest Jev fit on the
  roadmap.
- **Where it does not:** the proposal never becomes a contract. Only an author accepting it
  (editing the task to carry the annotation) makes it one.
- **Cost:** one decision call per unanchored task per converge run, under `[decide]` budget;
  zero when the provider is `none` (I42).
- **The gate:** before any proposal is shown, SI-M4 reads the port against an adjudicated
  sample of at least 60 foreign tasks with human-chosen anchors. The proposal path turns on
  only if the top choice is right at least nine times in ten at the confidence it is shown at
  (the route site failed this at 45.7%, 2026-09-24). Otherwise proposals are shown without a
  confidence, ranked by the deterministic `pack_context` score, or dropped.

## 7. Tenancy and cost

Detection walks only the paths each dialect names; parsing is proportional to spec files,
not the repository. The unclaimed-code pass reads the changed-file set of one range. No new
table: a foreign spec is parsed per verify run like a native plan.

## 8. Measurement

| Reading | Threshold |
|---|---|
| SI-M1 | 5 dialects verify in place on fixture projects (one per dialect, committed under `internal/specdialect/testdata/`), foreign files byte-identical after the run |
| SI-M2 | the converge report on a real Spec Kit or OpenSpec project, with the counts of each section and a human check of 20 items |
| SI-M3 | a plan under `Proof: certified` refuses a row whose certificate is `unproven` and accepts it once `live` |
| SI-M4 | Jev anchor calibration at least nine in ten at the shown confidence on at least 60 adjudicated tasks, or the drop recorded |

## 9. Refused

- Adopting a foreign format as canonical: none carries anchors.
- A spec authoring workflow, templates or commands of our own (the scaffolds own that).
- A separate SDD binary (non-goal).
- Writing outside the fathom converge block, or writing without `--write-fixups`.
- A model marking a task done.
- Reusing `tests:` for the test tier.
