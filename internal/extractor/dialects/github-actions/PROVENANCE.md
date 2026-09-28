# github-actions — provenance and divergences

LB-Q-github-actions, design `docs/design/language-breadth.md` §4, §13. This is a
doc-engine dialect (no tags.scm/queries pulled from an upstream grammar — the usual
target of this file, §2.1); it is recorded here instead for the port decisions, the
divergences from the incumbent, and the calibration oracle's provenance.

## Port of `internal/extractor/ghaextract.go`

`GHAWorkflow`/`GHAJob`/`GHAAction` are the incumbent's own `SourceKind` strings, kept
verbatim (§2.2, §13.2 step 15). `GHAActionDef`/`GHAStep`/`GHAActionInput`/
`GHAActionOutput` are new: the incumbent parsed only workflow files, never
`action.yml`/`action.yaml` (composite/reusable action definitions) or step `id:`s.

`TestGitHubActionsDialectIncumbentParity` (`internal/extractor/githubactionsdialect_test.go`)
checks the essential facts `ghaextract_test.go`'s own fixture established (a named
workflow, `needs:` edges, `uses:` steps captured, a `run:` step not captured as an
action) still hold for the new dialect — not a byte-for-byte reuse, because of the one
divergence below the doc engine cannot route around.

## Divergence: no file-basename name fallback

`ghaextract.go` falls back to the file's basename when a workflow has no `name:`. The
doc engine's selector language (design §4: "no expression language, no templating") has
no way to reach the file path from a value expression — `docValue` only composes
selector paths and `$key`, never a literal or the file name. A workflow without `name:`
therefore yields **no** `GHAWorkflow`/target entity at all (`workflow-no-name.yml`
fixture), rather than one named from the path.

This is not scored as a gap: the calibration oracle (below) also treats an unnamed
workflow's `name:` as its only identity and skips it the same way, so both sides agree
on what counts as a "target" definition. It would surface as a real `sym_recall` miss
against an oracle that *does* synthesize a name from the path (e.g. GitHub's own UI,
which shows the file path for an unnamed workflow) — worth knowing if a future
calibration subject or a stricter oracle is chosen.

## Divergence: `uses:` cross_file resolution is directory-relative, GHA's convention is repo-root-relative

GitHub's own semantics: `uses: ./path` in a workflow or `action.yml` is always relative
to the **repository root**, never to the referencing file's own directory. Every
`cross_file` strategy this engine has (`repo-path`, `image-ref`, `module-source`,
`service-name`; design §4, §6) resolves a `./`/`../` value relative to
`path.Dir(relPath)` — correct for GitLab CI includes or JS relative imports, wrong here.

Verified directly on both calibration subjects' own local `uses:` values (not a
hypothetical): a step `uses: ./.github/actions/other-helper` from
`.github/actions/my-helper/action.yml` resolves (via `module-source`) to
`.github/actions/my-helper/.github/actions/other-helper` — nested and wrong; the correct
target is `.github/actions/other-helper`. Every fixture with a local `uses:`/reusable
`uses:` value documents the actual (wrong) resolution in its golden, deliberately, so the
gap is visible in the test suite rather than hidden by a fixture that happens to live at
the repo root.

This is `needs-strategy` (design §13.2 step 13): no existing strategy fits, and adding
one (a repo-root-relative variant) is an Opus decision, not this worker's. The `uses:`
value and its raw/target/strategy are still recorded via `cross_file` — a future strategy
fix improves `target` without any dialect change.

## The calibration oracle

No SCIP producer exists for GitHub Actions, and no ground-truth-symbols mode exists in
`actionlint`'s CLI (`-format json` is lint diagnostics, not a symbol table — the queue
entry's "(parse only)" note). The oracle used here instead imports
`github.com/rhysd/actionlint` (v1.7.12) as a library — design §1.3 tier C, "an
independent parser library" — and walks its `Parse()` AST to emit a
`toolOracleFile` (`cmd/indexbench/langoracle.go`) directly: `Workflow.Name` -> `target`,
each `Job` -> `job` (+ `Needs` -> `references`), each step's `ExecAction.Uses` -> `resource`.
The generator (`ghaoracle`, not committed — a throwaway, per §13.4/§13.6's scratch-dir
convention) lives only in this worker's scratch directory; its own `go.mod` pins
`github.com/rhysd/actionlint@v1.7.12`, kept isolated from this module's `go.mod` (a
doc-engine dialect needs no new dependency, `pure-go:gopkg.in/yaml.v3` only).

Scope, disclosed: `actionlint.Parse` has no `action.yml`/`action.yaml` AST, so the oracle
never names a `variable_decl` (action input) or `output` (action output) definition.
Neither calibration subject (`actions/starter-workflows`, `cli/cli`) contains an
`action.yml`/`action.yaml` file at the pinned commit, so those two kinds are simply
unattested in this reading — extracted and golden-tested (`action-composite.yml`
fixture), never measured against real-world content.

## A shared foundation bug found and fixed here

`internal/langbench.Manifest.Owns` (the file-listing helper `cmd/indexbench/langoracle.go`'s
`measureSubject` uses to pick a subject's dialect files) checked only `Match.Basenames`
and `Match.Extensions` — `ManifestMatch` never even decoded `globs`. Every glob-only
dialect (a workflow file under `.github/workflows/` is a path shape, not a basename or
extension) measured **zero** files from `lang-oracle`, although
`internal/connector.ClassifyDialect` (production ingest) already classified them
correctly — the gap was isolated to this one bench-only helper. Fixed in
`internal/langbench/spec.go` (glob matching mirrored from `internal/connector`'s
`globMatch`/`globSegs`, same "`**` matches zero or more segments" semantics), covered by
`internal/langbench/spec_owns_test.go`. This was the first queue item needing a
glob-shaped `match` (every landed dialect before this one matched by basename or
extension alone), so the gap had never been hit.
