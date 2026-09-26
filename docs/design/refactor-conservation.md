# Refactor conservation — mechanics

Design for vision 34 (ADR-044). The plan (`docs/plans/refactor-conservation.md`)
is gated on this document. Every section names what is deterministic, what each
finding means, and which inputs must be present before a plane may say anything
other than `unproven`.

**Status (wave 3, 2026-09-24).** §1–3 (proof integrity, the ledger, absence/
anti-Blindness) landed and are proven. §4 (refactors as programs) and §5 (the
typed floor) shipped their CLI verbs (`fathom refactor plan`/`status`,
`fathom report evidence`). §6's pre-push wiring and the `review`/`review_diff`
`conservation` dimension are wired. §7's decision-port sites and §7a's
cross-repo test-kit/Review Kit rows remain `(op)`/unclaimed in the plan.
**CV-M6 (the dogfood refactor program) passes**, but only after fixing a
real gap it found: `fathom verify --store` had never wired a
`verify.GoneEvidence` adapter, so a `gone:` row (§3.1) could never be
proven true or false via the CLI, only ever `Unknown` — now fixed
(`cmd/fathom/verify_gone_evidence.go`,
`docs/bench/readings/cv-m6-dogfood-2026-09-24.md`). **CV-M4 (wave 3
gap-closing):** §5's own measured tiers first surfaced that the `surface`
plane's Go `api:` delta (§2.1) was a purely textual comparison, not a
type-identity one — CV-M4 fired the ADR's revisit trigger at 0.615p/0.941r
over 38 pairs against its 0.95/0.95 bar (three named disagreement classes:
`interface{}`/`any` false positives, additive-struct false positives, a
type-alias false negative —
`docs/bench/readings/cv-m4-apidiff-calibration-2026-09-24.md`). Fixed by
`internal/analysis/inventory/apicompat.go`: every Go `api:` type
declaration now carries a `TypeDetail` (struct field set, interface method
set, alias-vs-defined kind) built from the SAME per-file `go/ast` parse
Extract already does — no `go/types` pass, no second module load — and
`Diff` applies apidiff's own compatibility rules (§2.2 below) over it
instead of comparing rendered text; `any`/`interface{}` are canonicalized
at render time for every Go item, type or not. Re-measured on the full 50
pairs: precision 0.960, recall 1.000
(`docs/bench/readings/cv-m4-apidiff-full50-2026-09-24.md`). **CV-M5 (the
Blindness replay) now passes 2/2**, after fixing a real false-positive
class it found: §3.3's resurrection detector matched a deleted base
function against ANY function in a changed file at head sharing its body
hash, without checking whether that matching head function was itself
touched by the change — the genuine ADR-025 DuckDB removal produced 24
false `body_resurrected` findings this way, because `DuckStore` and
`SQLiteStore` had implemented the same `Store` interface with convergent
CRUD bodies for years. Fixed: a head function is now a resurrection
candidate only when it is itself new in the range or its body hash
changed in the range — a function whose identical body already existed at
base under its own name is never a candidate, retirement declared or not
(`internal/conserve/resurrection.go`,
`docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md`'s 2026-09-24
update). That replay also surfaced a second, unrelated gap — filed as
O117 (`.fathom/observations.md`), not fixed here — `fathom conserve`'s
default plan discovery hands absolute paths where its plan reader expects
repo-relative ones, so `retires:`/`surface_change:` declarations may be
silently invisible on the real repository whenever `--plan` is not passed
explicitly (the only way the pre-push gate invokes it today).

**Status (wave 3, gap-closing, 2026-09-25).** Three real gaps this program's
own dogfooding surfaced, all fixed: (1) `review_diff`'s (MCP) §1.6 wiring
built a real conservation `Input` and ran every plane for `conservation`
but never flattened the SAME "integrity" plane's own findings onto
`DimIntegrity`, so a resolved `since` rendered `checked:true` with zero
findings instead of the honest "needs a base ref" — `review`/`review_diff`
now carry BOTH `integrity` and `conservation`
(`internal/mcp/review_integrity.go`). (2) §6's activation clause (CV-41)
was inert in practice: a dry-run compile never resolved any price at all
(`WithLLM` ran only on a spending compile), so `fathom compile --dry-run`
always showed `cost_usd: est. 0.00-0.00` regardless of `--profile` —
`cmd/fathom/modelprice.go` gives a dry run a real per-model price when one
is known, and an honest "cost unknown: no price for model `<m>`" when it
is not. (3) A layer's pre-flight budget refusal fired on its OWN first
compile exactly like a re-compile, so CV-41's newly-activated
`adr-rules`/`spec-bindings` could not actually start on a repository
bigger than one run's budget without an operator already knowing to pass
an override — `semantic.CompileInput.NoPriorRun` skips the refusal only
for a layer's genuinely first compile_run, making real progress and
stopping as `budget_stopped` (resumable with `--continue`, unchanged for
every later re-compile). The fleet recompile this activation clause exists
for then ran clean: `docs/bench/readings/cv-m8-fleet-recompile-2026-09-25.md`.
(4) O117 (`.fathom/observations.md`) is fixed: `fathom conserve`'s default
plan discovery — the only invocation shape the pre-push gate uses — handed
`internal/conserve.NewInput` absolute `docs/plans/*.md` paths where its
git-based reader expects repo-relative ones, so every `retires:`/
`surface_change:` declaration was silently invisible without an explicit
`--plan`. `NewInput` now relativizes an absolute plan path against its own
`root` before reading it, so the resurrection plane's retirement gate
(§3.3) and the surface/certificates planes' base-side I45 excusal both see
declarations made anywhere in `docs/plans/*.md` with no `--plan` flag
needed.

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
then kind, then key. Every Finding additionally carries `file`/`line` (both
`omitempty`) — the finding's SUBJECT's own repo-relative location, at HEAD
when it still exists there, else at BASE — for a citation-grounded caller
(the Review Kit adapters) that needs a real source anchor and not just the
human-readable `base`/`head` strings. Populated wherever a plane already has
the location at hand or cheaply parsed (a guarded test's file:line, a
reappeared function's head site, a capability inventory item's declaration,
a `// certifies:` marker's own comment line); left empty, never invented,
when nothing names one (e.g. a `governance` claim's scope is a glob list,
never a single file).

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

`inventory.Diff(base, head)` yields `removed`, `changed` (same key, a
comparison over both sides' `detail` — or, for a Go `api:` type
declaration where both sides carry one, over the richer comparison below
instead — disagrees) and `added`. Losses are `removed` and `changed`. A
loss is declared when a plan row at base carries `surface_change: <key>`
or `retires: <key>` (prefix match with `*`), otherwise self-declared or
undeclared.

**apidiff agreement (CV-M4, wave 3 amendment).** A Go `api:` declaration
for an exported top-level `type` carries a `TypeDetail` alongside `detail`
(`internal/analysis/inventory/apicompat.go`) — struct field name -> type
text, interface method name -> signature text, and whether the
declaration is `type X = T` (an alias) or `type X T` (defined) — built
from the SAME `go/ast` parse `Extract` already does over each side's file
content (no `go/types` pass, no second module load: P19 stays intact).
When both sides of a key carry one, `Diff` applies
`golang.org/x/exp/apidiff`'s own compatibility.go rules instead of
comparing rendered text: an added exported struct field is never a loss
(only removing or retyping an EXISTING one is — apidiff's own
`checkCompatibleObjectSets`); an added interface method is a loss UNLESS
the base interface already carried an unexported method (apidiff's own
asymmetric `checkCompatibleInterface` — a sealed interface nobody outside
the package could implement anyway may grow for free); a `type X T` ->
`type X = T` flip (or the reverse) is always a loss, whatever the RHS text
does, since apidiff's own correspondence check treats a Named type and an
alias to a non-Named RHS as different type kinds outright. `any` and
`interface{}` are canonicalized to one spelling at render time
(`normalizeAnyType`) for every Go item, not only type declarations, so a
func/const/var's plain-text `detail` comparison collapses the two
spellings too. An interface's own embedded-interface fields are tracked
too (wave 3 gap-closing): adding, removing, or retargeting an embed (e.g.
`io.Closer`) registers as a change to the enclosing interface's shape,
via the same asymmetric compatibility rule a named method addition
already gets — a review of this fix found that `interfaceMethodShapes`
skipped every embed outright, so `Diff` reported `conserved` on a real,
apidiff-classified incompatible change the pre-CV-M4 whole-text `Detail`
comparison used to catch. What this does NOT do, on purpose (documented
gaps, not silent claims): FLATTENING an embedded interface's own method
set (resolving what `io.Closer` itself requires, rather than only
noticing that the embed's own identity changed), fields promoted through
an embedded STRUCT (apidiff's "exported *selectable* fields"), a
struct's comparability regression, and anything that needs another
package's declaration resolved (Go's numeric-widening compatibility
table, cross-package generic instantiation) — none of CV-M4's three
measured disagreement classes need cross-package resolution, so a
`go/types`/`go/packages` pass over the changed package's own transitive
import graph (apidiff's own cost, ~7–75s/pair on the calibration corpus)
would buy nothing here; that tier is where a FUTURE disagreement class
that genuinely needs it would have to land, not this one.

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

### 3.4 The reading set (the residue, Q22)

Decision 7 hands Review Kit "the residue no test covers" and decision 1 lets the
decision port order attention over it, but nothing computed the residue, so
`refactoring.uncovered-behaviour` had no evidence to judge. The reading set is
that computation: the changed functions in `base..head` whose behaviour no
commit-bound passing test vouches for. It is what a reviewer still has to open
after every plane has spoken.

**Changed functions.** For each changed `.go` file that is neither a test file
nor tagged generated (those belong to the integrity and generated planes),
`funcSitesIn` at base and at head (the parse resurrection already does). A head
function is `added` when no function of the same receiver and name exists at
base in the same package directory, and `modified` when one does and the exact
code text differs: the function's token stream with comments dropped, so a
comment, blank line or reflowed line inside the body is no more a change than a
doc comment above it (amended 2026-09-25 after CV-M12's reading listed a
comment-only `runLocalize` edit). A function whose body is unchanged but whose
file moved is neither. Deleted functions are not in the reading set: the compiler proves their
referents gone, and `gone:`/`retires:` rows and the resurrection plane cover the
rest.

**Classes.** Each changed function is looked up in the index at HEAD and walked
back over resolved call edges to `Test*` functions (`callgraph.TestImpactDetail`).
Then:

- `attested`: at least one reaching test's latest run passed, is bound to a
  commit, was made on a clean tree, and nothing in the test's file or the
  function's file changed since that commit (`certify.reachUnchangedSince`, the
  rule O114 gave certificates, applied without a declared target);
- `unattested`: tests reach it, but none has such a run;
- `unreached`: no test reaches it over resolved edges.

**Order and bounds.** Items list `unreached` first, then `unattested`; within a
class by resolved caller count descending, then symbol. The list is capped
(default 200, `--residue-max`); the report says how many were cut. No
percentage, ratio or threshold is computed (I43): counts and lists only, and
Review Kit judges.

**Honesty.** The reading set is `unproven`, with its missing inputs listed and
no function classified, when the index is not at HEAD, typed packages failed, or
the base cannot be read (I44). Changed files in a language without typed edges
are listed as `unanalyzed` by path: a reviewer reads them, and fathom says so
instead of implying they were checked.

**Where it is served.** `fathom.conserve/v1` gains an additive `residue` object
(`changed`, `attested`, `unattested`, `unreached`, `items`, `truncated`,
`unanalyzed`, `missing`); it is not a plane and never changes a plane's verdict
or `--strict`'s exit. `fathom conserve` prints the reading set after the planes.
The review `conservation` dimension carries the counts and the first five
`unreached` items. Review Kit's `fathom-conserve` adapter reports the residue as
the evidence for `refactoring.uncovered-behaviour`: fail when any item is
`unreached` or `unattested` (citing the first item's file and line), ok when
none, not-yet-checked when the residue is unproven. That requirement stays
advisory; its typed-judgment lane answers per item.

**The decision port.** The item list is exactly the "frontier" CV-51 needs.
Ordering it by a model is still deferred: CV-51 fires only when a real refactor's
reading set exceeds 100 items and a corpus exists to A/B the ordering against
the deterministic caller-count order. Asking a model whether a changed function's
behaviour is preserved is refused (decision 8: never safety).

**Kill criterion (CV-M12).** Measured on two real ranges of this repository (the
CV-M6 dogfood range and a CV wave range) with runs attested at each range's head:
the files the reading set names, plus the unanalyzed files, are fewer than the
range's changed files, and a hand check of 20 items finds each one a genuinely
changed function in the right class. If the reading set narrows nothing, or a
hand-checked item is wrong in a way the rule cannot fix, it comes out.

### 3.5 Execution evidence (CV-M13)

The reading set first judged a function `attested` when a passing, commit-bound
test *statically reached* it. CV-M13
(`docs/bench/readings/cv-m13-reading-set-calibration-2026-09-25.md`) measured
that rule against Go coverage and found it both too generous and too strict.
Over every function at one head of this repository, 168 of 4,534 `attested`
functions (3.7%) never executed: branches no test takes, the CLI verb switch,
interface fan-out, and MCP handlers credited to the `register*` function that
wraps them. 31 of 91 `unreached` functions did execute, called from outside the
first-party graph: the standard library through an interface (`flag.Value`,
`slog.Handler`, `Unwrap`, `MarshalJSON`, `String`), the runtime (`init`), and
package-level initializers. The test named as the voucher was always a hub.
Function-level execution was also coarser than the change: `runServeHTTP` ran,
but its one changed line (the O119 fix) never did.

**The profile.** `task tests:attest` runs the suite once as
`go test -json -coverpkg=./... -coverprofile=<results_dir>/coverage.out ./...`,
so the same run feeds both the envelope and the profile, and writes
`<results_dir>/coverage.meta.json` (`schema: fathom.coverage/v1`, `commit`,
`dirty`, `race`, `profile`, `passed` = no failed case). `TESTKIT_COVER=0` turns
the profile off. `fathom conserve` reads the meta from `[tests] results_dir`,
or from `--coverage-meta <file>`.

**Usable only when bound.** The profile is used only when its meta names a
commit, `dirty` is false and `passed` is true (I44). For each function, the
profile also counts only if neither the function's file nor its test's file
changed between that commit and HEAD, the same rule O114 gave certificates. An
unusable profile is named in the report (`coverage_unusable`), and every item
falls back to static reach. A profile that maps only partly (some file keys
match no module here) is still used; the unplaced keys are counted and sampled
(`coverage_unmapped`, `coverage_unmapped_sample`), since functions in those
files fall back to static reach. Each item records which evidence classed it
(`evidence`: `executed` or `static-reach`).

**Mapping.** Profile blocks are keyed by import path. They are mapped to
repository paths through the module path of the nearest `go.mod`, and duplicate
blocks across per-package sections are merged by maximum count. A function's
blocks are those inside its line span. Its *changed blocks* are the ones that
overlap a line `git diff -U0 base..head` added or changed inside the function;
for an added function, that is every block.

**Classes with execution evidence.**

- `attested`: every changed block executed. A modified function whose changed
  lines hold no block (a signature-only edit) has no changed block, so it is
  cleared.
- `partial`: the function executed, but some changed blocks never ran. The item
  lists those line ranges (`unexecuted_lines`).
- `unexecuted`: no block of the function executed.

Without usable coverage the static classes stand: `attested`, `unattested` and
`unreached`. Items list every class except `attested`, in this order:
`unexecuted`, `unreached`, `partial`, `unattested`. Within a class they are
sorted by resolved caller count, then by symbol. `tests` stays the static
reach, never "the test that executed it", because a merged profile cannot
attribute execution to a test. Counts and lists only (I43).

**Reconciling counts.** A changed function in a file the index does not analyze
(`vendor/`, an ignore rule, an unparseable file) is counted in
`unanalyzed_functions`. So `changed` always equals the sum of the classes plus
`unanalyzed_functions`, and a reader can see where every function went.

**Test identity.** `fathom tests ingest` joined cases to `Test*` symbols by bare
name. A name shared by tests in two packages joined to neither, so those tests
never attested or certified anything (CV-M13's control: 34 of 566 cases). When
the case id carries its package, the join uses it.

**The decision port.** Execution evidence is deterministic; Jev has no role.
Asking a model whether an unexecuted changed line is "safe" is refused under
decision 8.

**Kill criterion (CV-M14).** Re-run the CV-M13 ranges with coverage: fathom
5afeee42..78e508d3 and conjure's three measured ranges (r1, r2, r3). By
construction, no `attested` item may be unexecuted and no flagged item may
have executed. `runServeHTTP` must read `partial`, naming
`cmd/fathom/servehttp.go` line 131. The `slog.Handler` methods (only present
in conjure's r3, 44fd5ae..e9d0f2e) must read `attested`. `WithGroup` and
`ParseError.Unwrap` must read `unexecuted`. This prediction is meaningful only
against a bound, `passed: true` profile at the range's own head (I44: a
profile whose run did not pass is refused globally, and every item in that
range falls back to static reach regardless of what actually executed) — the
literal r3 head commit's own suite fails on an unrelated, date-sensitive test
unfixed until a later commit, so the operator measurement must re-run tests
against a tree where that fix is present (or any other tree at r3's head
whose suite genuinely passes), never treat a failing head as satisfying or
refuting this criterion. Separately, if an attest run with `-coverpkg=./...`
takes more than twice as long as the plain run on this repository, the
profile becomes opt-in (`TESTKIT_COVER=1`) instead of default.

### 3.6 What a reviewer still opens (Q22's answer, 2026-09-25)

Q22 asked what a reviewer must be shown to accept a large agent refactor without
reading the diff, and whether a stranger can re-derive it. The measured answer is
the reading set with execution evidence. CV-M12, CV-M13 and CV-M14 measured it on
overlapping ranges.

- **This repository's CV wave-3 range** (`5afeee42..78e508d3`: 47 changed files,
  31 changed Go functions). With a bound, passing coverage profile, 18 functions
  read `attested` and 13 read `partial`. None read `unexecuted`. Each partial item
  names the changed line ranges that never ran; for `runServeHTTP` that is lines
  129-133, the O119 fix (CV-M14). CV-M13's independent changed-line join found 13
  such functions on the same range: seven in
  `internal/analysis/inventory/apicompat.go`, three in
  `internal/mcp/review_integrity.go`, and `runServeHTTP`, `planPathRelativeTo` and
  `Resurrected`. The reviewer opens those 13 functions at their named lines. They
  also open the 21 changed files the reading set does not analyze: docs, the ADR,
  CHANGELOG, `.reviewkit/*.cue` and CODEOWNERS (CV-M12). That replaces opening all
  47 files.
- **Static reach alone was not enough.** CV-M12 classed all 31 of those functions
  `attested` and flagged none. Over the whole head, 168 of 4,534 functions that
  static reach called attested never ran, and 31 of 91 it called unreached did
  (CV-M13).
- **The CV-M6 dogfood range** (`c75b70e8..4957b916`: 41 changed files, 36 of them
  deletions). CV-M12's reading set named one changed function and 3 unanalyzed
  files. The function was a comment-only edit, which the amended rule in §3.4 no
  longer counts. The absence and resurrection planes cover the deleted files, so
  the reading set does not list them.
- **A weaker-tested repository.** On conjure's r2 range (326 changed functions; 100
  of them in vendored files the index excludes) the reading set flags 105 `partial`
  and 2 `unexecuted` of the 226 it classifies (CV-M14). Weaker tests make the
  flagged set larger; they do not make the change look reviewed. Across conjure's
  three ranges, CV-M13's hand check confirmed four changed functions never ran.
  Static reach called two of them `attested`: `jsonShapeError.Error` (r1 and r3)
  and `webui.handleAdoptStream` (r2, 163 reaching tests, 0.0% covered). It called
  the other two `unreached`: `ParseError.Unwrap` (r2) and `fanoutHandler.WithGroup`
  (r3). With coverage, CV-M14 cross-checked three of the four against the raw
  profile and each reads `unexecuted`: `jsonShapeError.Error` (r1's only
  `unexecuted` item), `ParseError.Unwrap` (r2) and `fanoutHandler.WithGroup` (r3,
  a supplementary reconstruction). CV-M14 does not name r2's second `unexecuted`
  item. CV-M13's r2 coverage join has exactly two functions at 0.0%,
  `ParseError.Unwrap` and `handleAdoptStream`
  (`docs/bench/readings/cv-m13-method/conjure/r2-classes-with-coverage.tsv`), but
  no reading records `handleAdoptStream`'s class under coverage.

**Re-derivation.** A stranger can re-derive the numbers for this repository's
ranges from the repository alone. Each reading carries its reproduce recipe, and
CV-M13's scripts are committed under `docs/bench/readings/cv-m13-method/`. CV-M14
cross-checked every named case against the raw `go tool cover` profile. The
conjure numbers need that repository, which a stranger does not have. No 400-file
range has been measured; the largest is conjure r2, with 326 changed functions.
On this repository, the evidence that flagged items are real rests on one range
(the 13 partial functions above). No seeded undertested range has been run here.

**What stays open.** Promoting `refactoring-conservation` from warn to deny is the
owner's decision, with this section as its evidence (`docs/intent/questions.md`
Q28).

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
  `fathom conserve --json --since <base> --service <service>` — `--service`
  is required (cmd/fathom's conserve subcommand has no default-from-
  repo-directory fallback the way `certify gaps`/`certify manifest`/
  `certify status`/`certify orphans` do), so the adapter is constructed
  with the sounding's own configured Service, the same value
  `NewFathomAdapterAtRoot` already threads through for `fathom verify`
  bindings — fails on a finding with `declared` other than `prior`),
  `refactoring.proof-integrity` (integrity
  findings of kind `deleted`, `weakened`, `skipped`, `golden_changed`,
  `marker_moved`), and `refactoring.uncovered-behaviour` (typed-judgment lane,
  advisory). Armed at warn on this repository. The testing-certifications
  requirement is armed at warn on this repository too, with the corrected kill
  rule.

## 8. Refused

A transformation engine; an ordinal rung or score; a full base re-ingest;
absence proofs for what the compiler proves; edit-time test freezing; model-made
move decisions or strength re-asks; deleting the `clone` package.
