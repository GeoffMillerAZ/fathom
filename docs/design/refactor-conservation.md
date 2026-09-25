# Refactor conservation — mechanics

Design for vision 34 (ADR-044). The plan (`docs/plans/refactor-conservation.md`)
is gated on this document. Every section names what is deterministic, what each
finding means, and which inputs must be present before a plane may say anything
other than `unproven`.

## 0. Vocabulary and the change range

- **Base**: a git ref resolved to a commit (`--since <ref>` or a plan's
  `baseline:`). **Head**: the working tree (or `--head <ref>`).
- **Changed files**: `git diff --name-only <base>` plus untracked files, the same
  set `fathom review --since` already computes. Base-side content is read with
  `git show <base>:<path>`; nothing is re-ingested at base (P19).
- **Plane**: one family of checks. Planes: `integrity`, `surface`,
  `certificates`, `governance`, `absence`, `resurrection`, `generated`.
- **Finding**: `{plane, kind, key, detail, base: file:line, head: file:line,
  declared: none|prior|self, remedy}`. `declared: prior` means a declaration
  present at base excuses it; `self` means the declaration was added inside the
  range (I45) and excuses nothing.
- **Plane verdict**: `conserved` (checked, no undeclared finding), `losses`
  (at least one finding not declared at base), or `unproven` (an input was
  missing; the missing inputs are listed). Never a score.

The JSON report is `fathom.conserve/v1` (compatibility surface):

```json
{"schema":"fathom.conserve/v1","service":"fathom","base":"<sha>","head":"<sha|worktree>",
 "planes":[{"plane":"surface","verdict":"losses","missing":[],"findings":[{...}]}],
 "tiers":[{"language":"go","tier":"typed"},{"language":"python","tier":"file-resolved"}]}
```

Field names and plane/kind strings are the wire contract; sort order is plane,
then kind, then key.

## 1. Stage 1 — proof integrity

### 1.1 Commit-bound runs (fixes O114)

- test-kit `#EnvInfo` gains optional `commit` (40-hex), `dirty` (bool) and
  `race` (bool) — the commit the suite ran against, whether the tree had
  uncommitted changes, and whether the race detector was on. The go-test-json
  adapter fills them from `git rev-parse HEAD` and `git status --porcelain`. An
  in-toto subject named `gitCommit` is accepted as the same fact.
- fathom mirrors the schema (`schema/testkit/`, drift-gated), and
  `testevidence.Run` / `store.TestRun` carry `Commit`, `Dirty`, `Race`
  (additive columns).
- `certify.CertificateStateAt(reached, runs, reachUnchangedSince)` replaces the
  commit-blind rule. `reachUnchangedSince(commit)` reports whether no file in the
  certificate's reach (its test file plus the files of the symbols it reaches)
  changed between `commit` and HEAD, including uncommitted edits. Rules, in
  order: unreached → `unreachable`; no runs → `unproven` ("no attested run");
  newest run failed → `broken`; newest run unbound (`commit` empty or `dirty`) →
  `unproven` ("run not bound to a commit"); reach changed since the newest
  passing bound run → `unproven` and the claim's status `stale` ("reach changed
  since the attested run at <sha>"); a flake in the last three → `flaky`;
  otherwise `live`.
- The sweep uses the same function, so a changed reached symbol stales the
  certificate (the half of TC-8 that O114 found missing).
- Consequence: existing certificates read `unproven` until runs are re-attested
  with a commit. `task tests:attest` does that for this repository (runs
  `go test -json`, converts with test-kit's adapter when it is installed, and
  ingests with `fathom tests ingest`).

### 1.2 Guarded tests

A test is guarded when, at base, it carries a `// certifies:` marker, is named by
a `#Certifies` entry, is named in `docs/intent/invariants.md`'s enforcement
column, is named as Evidence on the compatibility surface in
`docs/intent/scope.md`, or reaches a changed symbol (when an index is
available). `integrity.GuardedTests` returns the set with the reason for each.

### 1.3 Test integrity findings (Go; other languages report `unproven`)

For each guarded test, parse its function at base (`git show`) and at head.
`integrity.NormalizedBodyHash` hashes a function body's AST with every
identifier replaced by a positional placeholder (consistent within the body)
and literals kept, so a pure rename leaves the hash unchanged.

| kind | when |
|---|---|
| `deleted` | the function is gone at head and no head test in the same package has the same normalized body |
| `renamed` | gone under its name, found at head under another name by normalized body (reported, not a loss unless a `-run` pattern, marker or enforcement cell still names the old name) |
| `weakened` | the count of assertion calls (`t.Error*`, `t.Fatal*`, `t.Fail*`, and `assert.*`, `require.*`, `cmp.*`, `is.*` selectors) fell |
| `skipped` | a `t.Skip*` call or a `testing.Short()` guard was added, or a `//go:build` line was added to the file |
| `golden_changed` | a `testdata/` path named in the test body changed in the range |
| `marker_moved` | its `// certifies:` marker was removed, or now sits on another function |
| `edited` | the normalized body changed but none of the above apply (informational; never a loss on its own) |

Everything is computed over base..head as a whole, so an edit in one commit and
the code change in another are caught together.

### 1.4 Self-declaration (I45)

`verify.AnnotateSelfDeclared(current, atBase)` compares a plan at head with the
same file at base. A row is self-declared when, relative to base, it gained
`skip_test`/`skip`, a `disposition`, a `retires:` or `surface_change:` key, or a
raised `max:`; rows present at base and missing at head are reported as
`deleted_row`. `fathom verify --since <base>` and the conservation report render
these as `self-declared`; a self-declared skip on a claimed row fails the row,
exactly as O56's check does today. The existing one-prior-version check stays for
callers without a base.

### 1.5 Generated files

A changed file the connector tags as generated ("Code generated … DO NOT
EDIT") whose generator inputs (same-directory `.proto`, `.cue`, `.sql`, or a
`//go:generate` source named in the package) did not change in the range is a
`generated`/`hand_edit` finding.

### 1.6 Where it is served

`fathom review --since <base>` and `review_diff` (new optional `since` argument;
without it the dimension says `checked: false, reason: "needs a base ref"`) gain
an `integrity` dimension; `fathom conserve` includes the plane.

## 2. Stage 2 — the conservation ledger

### 2.1 Capability inventory

`inventory.Extract(files)` returns items `{key, kind, file, line, detail}` from
file contents (so the same code runs on base and head content):

| key | source |
|---|---|
| `api:<importpath>.<Name>` and `api:<importpath>.<Type>.<Method>` | Go exported top-level declarations in packages that are neither `internal/` nor `main`; `detail` is the signature with parameter names removed |
| `json:<importpath>.<Type>.<tag>` | JSON tags on exported fields of exported Go structs |
| `route:<METHOD> <path>` | `internal/analysis/routes` |
| `env:<KEY>` | `internal/analysis/envkeys` |
| `rpc:<package>.<Service>/<Method>` | `.proto` service definitions |
| `cli:<verb>` | cobra `Use:` fields |
| `<kind>:<name>` | configured patterns: `[[surface.pattern]] kind glob regex` in `.fathom/config.toml` (this repo declares `tool:` for MCP tool names and `verb:` for CLI verbs) |

### 2.2 Delta

`inventory.Diff(base, head)` yields `removed`, `changed` (same key, different
`detail`) and `added`. Losses are `removed` and `changed`. A loss is declared
when a plan row at base carries `surface_change: <key>` or `retires: <key>`
(prefix match with `*`), otherwise self-declared or undeclared.

### 2.3 Cross-repo consumers

For a removed or changed `route:` key, service-reference edges in the store whose
target matches name the calling service and file:line (`kind: consumer_loss`).

### 2.4 Certificates plane

- `certify.LostDeclarations`: a certifies declaration (marker or manifest) at
  base whose target has no declaration at head and is not retired → `lost`.
- For certificates whose test or reach is in the range, their current state from
  the store: `unproven`, `broken`, `unreachable` → findings with the state's reason.
- `behaviour` targets resolve to inventory keys at head
  (`// certifies: behaviour route:GET /v1/x (golden)`); an unknown key is
  `contradicted`, so characterization and golden tests finally certify something
  checkable.

### 2.5 Governance plane

`conserve.GovernanceOrphans`: a compiled claim (obligations, adr-rules,
spec-bindings, findings) whose scope globs matched at least one file in
`git ls-tree -r <base>` and match none at head → `orphaned`.

## 3. Stage 3 — absence and anti-Blindness

### 3.1 `gone:` plan rows

| form | proves |
|---|---|
| `gone: import <importpath>` | no Go file at head imports it |
| `gone: module <path>` | `go.mod` at head has no `require` for it |
| `gone: pattern "<regex>"` with optional `paths:` | zero lexical matches (the lexical oracle's `max`, `min` and `eq` keys are now evaluated) |
| `gone: callers <Symbol>` with `package:` | the symbol is kept but has zero referents at typed tier; test referents counted separately and must also be zero unless `tests: allowed` |

Preconditions for a `proven` verdict: the index is at HEAD, the typed pass has
zero failed packages in scope, and no file excluded by build tags references the
target (a heuristic scan of excluded files). Otherwise the row is `Unknown`
("proven gone" never comes from a stale or partial index). A `max: N` on a
`gone:` row makes it a ratchet (stage 4).

### 3.2 Deprecated ratchet

`conserve.DeprecatedCallers`: symbols documented `Deprecated:` at head; a call
site to one that is new in the range (present in a changed file at head, absent
at base) → `absence`/`new_deprecated_caller`.

### 3.3 Resurrection

`clone.FuncBodyHash` hashes a function body identifier-insensitively (the
`clone` package's normalisation, exposed per function). `conserve.Resurrected`:
a function present at base in a changed file and gone at head whose hash matches
a head function under another name, with at least 5 statements, is a candidate.

**Resurrection honesty (amended 2026-09-24, owner decision).** An ordinary
rename or move of a function must not be a loss — most reappeared bodies are
exactly that, and nothing was ever claimed about them. A candidate is a loss
ONLY when the deleted function, its package, or an import it lived behind was
declared retired: a `retires:` key naming the function (`api:<dir>.<Name>`,
matched exactly or by a `"<prefix>*"` wildcard, same rule as `Declarations`),
or a `gone: import <path>` row whose `<path>` is, or ends in `/`+, the
function's own package name — from the plan(s) at BASE OR HEAD. Unlike
`Declarations`' I45 prior/self split (which excuses a loss only when the
declaration predates the range), a retirement declared inside the very same
range that deletes the code still counts as a claim here: that is exactly the
shape of the failure this check exists to catch — an agent writes the
`gone:`/`retires:` row a refactor program (§4) expects, satisfying the
surface-absence gate, while the body quietly survives elsewhere. When no
retirement was ever claimed, the finding is `resurrection`/`moved`, `Declared:
prior` — informational, never a loss (a normal rename needs no declaration
and pays no penalty for one it never made). When a retirement WAS claimed and
the body reappears anyway, the finding is `resurrection`/`body_resurrected`,
`Declared: none` — a loss: the declared retirement was false. This is the
deterministic Blindness detector.

## 4. Stage 4 — refactors as programs

- Plan header `baseline: <sha>` pins the base for every row.
- `fathom refactor plan <symbol|package|import>` writes a grammar-v2 plan: the
  baseline, one `gone:` row per package that still references the target (with
  `max:` at today's count), `certified_by` rows for certified targets the change
  reaches, the final `gone:` row, and a `conserve` row.
- `fathom refactor status --plan <file>` reports each ratchet's count now against
  its `max:` and the baseline, and new references that landed on main since the
  baseline (drift).
- Loosening a `max:` inside the range is self-declared (1.4).

## 5. Stage 5 — the typed floor beyond Go

- The file-resolved tier (SU-11) is shipped; the language tracker says so.
- The free L0 oracle measures precision and recall per language at the scip and
  file tiers on calibration clones; results are published in `docs/bench/`.
- Every conservation report names the tier per language touched; a plane whose
  facts rest on heuristic edges for a language is `unproven` for absence proofs.
- `fathom report evidence` gives, per service: typed share of call edges,
  certificate density (live over declared targets), inventory item counts, and
  whether receipts re-derive. It never ranks languages.

## 6. Layers and packs

- **Claims move with the code.** The sweep re-anchors a citation whose lines
  moved when its anchor text occurs exactly once elsewhere in the same file; the
  claim stays fresh with the new lines. Otherwise it goes stale as today.
- **Activation.** The default compile set includes `adr-rules` and
  `spec-bindings` when their corpus globs match at least one file; a fleet
  recompile runs for services with no layers or fully stale layers, dry-run
  priced first, with the spend recorded.
- **Obligations recall** is re-evaluated under the production model and fixed to
  clear its 0.75 bar; the findings eval is made to score.
- **Future direction, not built:** architectural decisions compiled into
  checkable dependency rules the call graph refutes (futures F24).

## 7. The decision port

- `[decide] sites` lists the sites allowed to send questions off-host
  (`route`, `compile_prefilter`, `obligations_rerank`, `certify_suggest`); a site
  not listed makes no call. Default: all current sites (behaviour unchanged).
- The calibration check (is 0.9 right nine times in ten on adjudicated cases)
  runs over logged route decisions against their cells' gold sources and over
  reviewed certificate suggestions; if it fails, the proceed band rises for that
  site.
- Frontier ordering and same-concept questions stay futures with triggers (F23).

## 7a. Review Kit and test-kit

- test-kit: the envelope fields in 1.1, the adapter filling them, an ADR in its
  own repository.
- Review Kit: pack `refactoring-conservation` with requirements
  `refactoring.no-undeclared-loss` (adapter `fathom-conserve` over
  `fathom conserve --json --since <base>`, fails on a finding with
  `declared` other than `prior`), `refactoring.proof-integrity` (integrity
  findings of kind `deleted`, `weakened`, `skipped`, `golden_changed`,
  `marker_moved`), and `refactoring.uncovered-behaviour` (typed-judgment lane,
  advisory). Armed at warn on this repository. The testing-certifications
  requirement is armed at warn on this repository too, with the corrected kill
  rule.

## 8. Refused

A transformation engine; an ordinal rung or score; a full base re-ingest;
absence proofs for what the compiler proves; edit-time test freezing; model-made
move decisions or strength re-asks; deleting the `clone` package.
