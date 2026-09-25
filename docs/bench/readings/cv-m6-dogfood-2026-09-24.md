# CV-M6: the dogfood refactor program (2026-09-24)

**Verdict (plain language). CV-M6 passes, but only after fixing a real gap
the program itself walked straight into.** The refactor program — retiring
this repository's own dead `internal/analysis/*` packages using `fathom
refactor plan`/`fathom conserve`/`fathom verify` — finished with every one
of its 18 rows proven `[x]` and a clean `fathom conserve --strict` report
(every plane `conserved`, zero findings, zero missing inputs). But getting
there required fixing a tooling defect discovered mid-program: `fathom
verify --store` had never wired an evidence adapter for `gone:` rows, so
every `gone:` row in this repository — including the ones this very plan
needed — read `Unknown` forever, no matter how true the claim. That gap was
already named in three places in this repository's own docs
(`cmd/fathom/refactor_plan_test.go`, `docs/usage.md`,
`docs/plans/README.md`) as "a later row." CV-M6 turned out to be that row.
Two more findings, both about the limits of "prove it unused" itself: an
import scan cannot see a plan file's own symbol claim, and it cannot see a
test elsewhere that depends on a package's file existing with specific
content — two of the twenty candidates the scan surfaced failed exactly
those checks and were correctly excluded rather than deleted.

Bar (CV-M6, `docs/plans/refactor-conservation.md`): "retiring this
repository's unused analysis packages runs as a refactor program and
finishes with every row proven and a clean conserve report." **Result: 18
of 20 zero-Go-importer candidates retired, every row proven, `fathom
conserve --strict` clean — the bar is met, on the corrected set.**

## What ran

1. **Find candidates.** `go list -json ./...` across the whole module,
   comparing every `internal/analysis/*` package's import path against
   every OTHER package's `Imports`/`TestImports`/`XTestImports` — 20 of 46
   packages had zero non-test importers. `clone` (the Blindness detector)
   and every wave 1-2 package (`routes`, `envkeys`, `apisurface`,
   `integrity`, `inventory`, `archconformance`, `serviceref`, …) had real
   importers and were never candidates. Cross-checked against a
   comprehensive text grep of the whole tree (`.go`/`.md`/`.toml`/`.cue`/
   `.sh`/`.yml`, third_party excluded) for each candidate's import path, to
   catch a reference an import scan cannot see (a plan's own symbol claim,
   a doc's file:line citation, a test's literal fixture text).
2. **Exclude two candidates the scan cannot safely retire** (below).
3. **Generate one plan row per remaining candidate** with `bin/fathom
   refactor plan <import path>` — 18 separate invocations, each target's
   own unedited output merged under one `Baseline:` header into
   `docs/plans/retire-unused-analyzers.md`, committed alone
   (`c75b70e8`, ADR-044 §4/I45: the plan's own absence declarations must
   predate the change they excuse).
4. **Discover the CLI could not prove a `gone:` row at all** (below), fix
   it test-first, commit separately (`b5540cb1` — landed BEFORE the
   deletion, once discovered, since nothing downstream could be proven
   without it).
5. **Delete the 18 packages** (source + tests + testdata) in one commit
   (`5068356f`), fixing the two doc/comment references that would
   otherwise cite a deleted file (`internal/core/model.go`'s `EdgeKind`/
   `Edge` doc comments, `internal/mcp/localize_tool.go`'s recency-hop
   comments naming `internal/analysis/churn`) and annotating a third that
   cited a deleted file:line as historical (`docs/plans/review-verb.md`'s
   `license_policy`/`security_rollup` audit rows).
6. **Prove and flip.** Fresh `fathom ingest` against the deletion commit,
   `fathom verify` proved all 18 rows, flipped to `[x]` in a follow-up
   commit (`4957b916`) — verify itself refuses to let the flip's own
   uncommitted edit count as proof (see "index at HEAD," below), so the
   authoritative proof is the ingest+verify run AFTER that commit landed,
   not before it.
7. **Full gate**: `go build ./...`, `go vet ./...` (and
   `CGO_ENABLED=0 go vet ./internal/store/`, untouched but checked),
   `gofmt -l` on every changed file (clean throughout), `go test -race
   ./...` (all packages pass, no regressions, no flakes), `fathom conserve
   --since c75b70e8 --json --strict` (every plane `conserved`), `fathom
   refactor status --plan docs/plans/retire-unused-analyzers.md` ("no
   ratchet rows found" — correct: every row is the final, unscoped
   absence proof, none of these packages had an importer to ratchet down
   from), and a hermetic verify of every plan in `docs/plans/*.md` using
   the SAME non-strict, per-plan, `--quiet` invocation `lefthook.yml`'s
   own pre-push gate uses — zero failures, zero operational errors, across
   all 63 plans including the new one.

## Finding 1 — the CLI could never prove a `gone:` row (fixed)

`internal/verify.EvaluateGone` (CV-27/CV-28, proven at the unit level
since wave 1) type-asserts its `Evidence` for an optional
`verify.GoneEvidence` capability; `cliEvidence`
(`cmd/fathom/genstatuscue.go`), the adapter `fathom verify --store` has
used since before this wave, implemented `Evidence`/`DetailedEvidence`/
`FileEvidence`/`CertificationEvidence` — never `GoneEvidence`. Every
`gone:` row therefore fell into `verify.go`'s own honest fallback: `Status:
Unknown, Detail: "no gone evidence available on this evidence source"` —
regardless of whether the claim was true. This was not a hidden bug: three
places in this repository already said so —
`cmd/fathom/refactor_plan_test.go`'s `fakeGoneEvidence` comment ("even
though no `GoneEvidence` CLI adapter is wired yet — a later row"),
`docs/usage.md` ("the row already passes the moment a `GoneEvidence` CLI
adapter exists to check it"), and `docs/plans/README.md` ("`fathom
verify`'s own CLI adapter for this is a later row"). A refactor program
built entirely of `gone: import` rows — exactly CV-M6's shape — was the
first real consumer to actually need that row.

Fixed test-first (`cmd/fathom/verify_gone_evidence_test.go`, three cases:
proves absence, never falsely passes a real importer, never falsely passes
a dirty working tree; all three red before the fix, green after) in
`cmd/fathom/verify_gone_evidence.go`. The design choice worth naming: three
of the six `GoneEvidence` methods (`GoFiles`, `CorpusFiles`, `ModuleFile`)
read the LIVE corpus (`client.Sources` re-reads disk on every call), so
they are always current regardless of when the store was last ingested.
The other three (`IndexAtHead`, `FailedTypedPackages`, `TypedCallers`)
depend on the STORE's last typed-edge pass, which genuinely can go stale
between an ingest and a later edit — so `IndexAtHead` requires the store's
recorded ingest commit to equal current git HEAD AND the whole working
tree to be clean, mirroring the exact refusal `fathom refactor plan`
already applies to its own `max:` seeding (`cmd/fathom/refactor_plan.go`).
Never a fabricated pass.

**What this made awkward.** `IndexAtHead`'s whole-tree-clean requirement
means a `gone:` row cannot be checked mid-edit, before anything is
committed — exactly the moment this session's own flip-to-`[x]` commit
(`4957b916`) hit: flipping a checkbox is itself an uncommitted edit, so a
`--strict` verify run against the just-edited, not-yet-committed plan
correctly read every row `Unknown` again ("the working tree has
uncommitted changes since the index was ingested") until that flip was
itself committed and re-ingested. The gate's own two-step shape (edit,
commit, THEN prove) is the only way to use this honestly; a workflow that
wants to preview a flip before committing cannot, today.

## Finding 2 — an import scan cannot see a plan's own claim or a test's fixture text

Two of the twenty zero-Go-importer candidates could not safely be
retired, and neither failure is visible to `go list -json`, `safe_delete_
check`, or any symbol/import-based tool:

- **`internal/analysis/migrationtargets`** — its `Find` symbol is claimed
  by an already-`[x]` row in `docs/plans/pulled-tools-excellence.md`
  (`id: P1-6`). Deleting the package would fail THAT plan's own gate
  (`symbol_present: claimed-but-absent`) the next time it is verified,
  without this change touching that plan at all.
- **`internal/analysis/packagedeps`** — zero Go importers, but
  `internal/scenariobench/guardsfathom_probe_test.go`
  (`TestGuardsI1ConsumersDoubleRunDetectorIsNameless`) `git archive HEAD`s
  the whole repository into a disposable copy and then reads and rewrites
  `internal/analysis/packagedeps/packagedeps_test.go`'s literal committed
  text, byte for byte, as a fixture for a guard-corpus regression probe.
  Deleting the package would make that test fail with a plain file-not-
  found the next time it ran against a committed tree missing the file —
  a real, currently-passing test, broken by a change a pure import scan
  would call perfectly safe.

Both are named, with their reasons, in `docs/plans/retire-unused-
analyzers.md` itself, and both are themselves evidence for CV-M6's own
premise: an import/symbol scan proves CODE is unused; it says nothing
about whether a PLAN FILE claims a symbol exists, or whether some other
test depends on a file's literal content rather than importing it. A
refactor program that trusted the scan alone would have broken one gated
plan and one passing test. `bin/fathom refactor plan`'s own generator
already excludes `_test.go` files from its ratchet-row scan for a related
but different reason (it counts PRODUCTION-code importers, not a
package's own black-box test); the verify-time absence proof correctly
does NOT exclude test files, which is why every one of the 18 retired
packages read `Unknown`-then-`Fail` (own external test package still
"imports" itself) until the whole directory, tests included, was actually
deleted — a useful sanity check that the new adapter was reading real
content, not a defect.

## What this shows

CV-M6's own premise held: fathom already had every fact this refactor
needed (`callgraph`'s typed callers, the plan grammar's `gone:` contract,
`conserve`'s planes), but nothing had yet exercised the CLI proof path for
a plan built entirely of absence rows, and nothing exercises the
distinction between "unused by import" and "unclaimed by anything else in
the repository" until a real program tries to act on it. Both gaps this
reading found are the kind CV-M6 exists to catch — a program is a better
test of the tooling than any unit test of the tooling in isolation.

## Reproduce

```sh
cd <fathom worktree>
go build -o bin/fathom ./cmd/fathom
bin/fathom ingest --store workspace/tmp/cv.sqlite --service fathom --path .
bin/fathom verify --service fathom --plan docs/plans/retire-unused-analyzers.md --store workspace/tmp/cv.sqlite --strict
bin/fathom conserve --since c75b70e8 --json --strict --service fathom --store workspace/tmp/cv.sqlite
bin/fathom refactor status --plan docs/plans/retire-unused-analyzers.md
```
