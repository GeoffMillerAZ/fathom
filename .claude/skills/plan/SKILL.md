---
name: plan
description: >
  Scaffold a gate-ready fathom plan in grammar v2. Use when the user asks to
  write, draft, or start a new plan/checklist under docs/plans/, turn a design
  doc into a work order, or convert a rough task list into something
  `fathom verify` can gate. Produces docs/plans/<slug>.md with stable IDs,
  {symbol, package} contracts grounded via pack_context/test_impact, typed
  dispositions for skipped work, and the verify command to run before any box
  is checked — so plans are born gate-ready instead of over-claiming later.
---

Plans are contract-bearing Markdown checklists, not prose. `docs/plans/README.md`
is the grammar's source of truth (§ conventions live there — read it if this
skill and the README ever disagree, the README wins). This skill exists so a
new plan is born in the shape `fathom verify` can gate, instead of needing a
rewrite pass after the fact (docs/vision/04 §4 item 2: "make plan mode emit
contracts by default").

## Before writing a single line

1. Pick a slug and confirm the file doesn't already exist:
   `docs/plans/<slug>.md` (kebab-case, e.g. `o8-precision-and-speclines.md`).
2. Ground every claimed symbol BEFORE it goes in a contract — don't guess
   package paths or symbol names from memory:
   - `pack_context` / `find_symbol` to confirm a symbol exists and see its
     real package directory.
   - `test_impact` (or `references`) to see whether a `Test*` already
     statically reaches it — this tells you up front whether `skip_test: true`
     will be honest or a cover-up.
3. If the plan implements/verifies a spec, get the spec's bracketed ID(s)
   (`[US-042]`, `[EP-013]`, `[FR-7]`, `[NFR-1]`, or a milestone form like
   `M18.2`) — `spec_refs` or `traceability` will confirm the ID is real.

## Checklist format (see docs/plans/README.md for the full grammar)

```
- [ ] Title text [SPEC-ID] {id: T-1, symbol: Sym, package: dir/of/sym}
```

- **Checkbox**: `[ ]` open, `[x]` claimed-done (gated — must verify),
  `[~]` in progress (never gated, never claim this as done), `[-]` skipped
  (exempt — needs `disposition:`). Never hand-write a `[x]` for something
  you have not actually grounded against the index.
- **Title**: human description of the work — keep it, spec IDs, and the
  `{…}` annotation ALL on one line. The parser is line-based: an annotation
  on a wrapped/continuation line is silently ignored and the task verifies as
  `no-contract` instead of carrying its intended contract. However long the
  line gets, keep it one line.
- **Spec IDs**: bracketed references tying the task to its governing spec,
  e.g. `[US-042]`. Omit only for genuinely spec-less housekeeping tasks.

## Annotation block — one `{id, symbol, package}` per line, always

Every gated (`[ ]`/`[x]`) line gets its own annotation with, at minimum:

- `id`: a **stable**, author-owned task ID (`T-1`, `OP-3`, …) — never rely on
  the positional `md-NNN` fallback, which renumbers when lines are inserted
  above it. Pick an ID scheme once per plan and keep it monotonic as you add
  tasks; never reuse or renumber an existing ID.
- `symbol`: the exact symbol name the task claims to touch, as it will
  actually exist in the index (confirm via `find_symbol`/`pack_context`
  above — don't invent it).
- `package`: the repo-relative directory the symbol's file lives under.
  Required whenever the bare symbol name could exist in more than one
  package — it disambiguates instead of leaving the claim to first-match
  luck (a still-ambiguous name after filtering resolves to nothing, honestly,
  never a guess).

Add `reachable_from` (an entrypoint the symbol must be statically reachable
from) and `min_tier` (defaults to `verified`) when the plan's contract needs
them. Loosen defaults explicitly — the zero value of every key is the
strictest contract.

## Skips need a typed disposition, always

A `[-]` line is exempt from gating, but it must say WHY, machine-visibly:

```
- [-] Title [SPEC-ID] {id: T-9, disposition: deferred, reason: "needs a keyed shell", trigger: "operator runs X"}
```

- `disposition`: `refused` (won't do — argue it here) or `deferred`
  (will do later — `reason` should point at what unblocks it).
- `reason`: free text justification — **double-quote** any value containing a
  comma (`reason: "kit-side done, donor NOT deleted"`); unquoted values must
  be comma-free.
- `trigger`: the revival condition for a `deferred` task (recorded/surfaced,
  not machine-evaluated in v0 — write it anyway, it's the thing a future
  session greps for).

## `skip_test` always needs a `skip:` justification

`skip_test: true` exempts a symbol from the "some `Test*` statically reaches
it" requirement. It is the gate's biggest honesty risk — a plan that waives
its test criterion most of the time is a demo, not a product (docs/vision/04
§4 item 1). So every `skip_test: true` carries `skip:` explaining why no test
can reach it (non-Go deliverable, generated code, a doc/skill file, etc.):

```
- [ ] Prompt v3 rewrite {id: OP-1, symbol: obligationsPromptVersion, package: internal/semantic, skip_test: true, skip: "prompt text change; identity rotation is the test"}
```

Never add `skip_test: true` just to make a claim pass — if in doubt, run
`test_impact` first and only skip if it's genuinely untestable by call-graph
reachability.

## Before checking any box

A `[ ]` becomes `[x]` only after this passes for the plan's service:

```sh
bin/fathom verify --service <svc> --plan docs/plans/<slug>.md
```

(CI additionally runs `--strict --store <tmpstore>`, which also fails
`Unknown`/no-contract claimed tasks — write contracts as if `--strict` is
always watching.) Read the per-task output before flipping a checkbox: a
`fail` means the claim is wrong, not the gate. Fix the claim (or the code),
never the checkbox alone.

## Worked skeleton

```markdown
# <slug> — one-line purpose

- [ ] First unit of work [US-042] {id: T-1, symbol: DoTheThing, package: internal/thing}
- [ ] Second unit, non-Go deliverable {id: T-2, symbol: cmdSomething, package: cmd/fathom, skip_test: true, skip: "shell-script glue, no Test* can reach it"}
- [-] Nice-to-have, not this wave {id: T-3, disposition: deferred, reason: "needs design review", trigger: "design doc lands"}
```
