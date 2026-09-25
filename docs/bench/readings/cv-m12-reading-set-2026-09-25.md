# CV-M12: the reading set measured on two real ranges (2026-09-25)

**Verdict (plain language). CV-M12 PASSES on both measured ranges, with a
caveat that limits what this particular reading can vouch for.** On the
CV-M6 dogfood range (18 packages deleted, 41 changed files) the reading
set named 3 files a reviewer still had to open — the reading set's own
zero flagged functions plus 3 unanalyzed docs — against 41 raw changed
files. On a CV wave-3 gap-closing range (47 changed files, 31 changed Go
functions) it named 21 — again the reading set's own zero flagged
functions plus 21 unanalyzed docs/CUE files — against 47 raw changed
files. Both satisfy the bar's "fewer than the changed files" clause. A
hand check of 20 items spanning both ranges found every one a genuinely
changed function, correctly classed. **The caveat: on both ranges every
single changed function came back `attested` — zero `unattested`, zero
`unreached`.** The hand check therefore could not exercise the "mix of
classes" the bar's own phrasing anticipates, and this reading cannot
speak to whether the reading set correctly flags a genuinely undertested
change — only that on two well-tested, gate-passing ranges it correctly
reports nothing to flag, with no false "attested" found in the sample.
For a reviewer, that is a real, useful signal (a clean bill of health
that took seconds to produce instead of reading 41 and 47 files), but it
is the easy side of the measurement; the hard side (catching a real
uncovered function) stays unmeasured here — see Caveats.

Bar (CV-M12, `docs/plans/refactor-conservation.md`): "on the CV-M6 dogfood
range and one CV wave range, with runs attested at each head, the reading
set's files plus unanalyzed files are fewer than the changed files, and a
hand check of 20 items finds each a genuinely changed function in the
right class; else the reading set comes out."

## Method

Built the NEW fathom (this checkout, branch `reading-set`, the commit that
carries CV-60..65) once: `go build -o rs-tools/fathom ./cmd/fathom`. Built
test-kit's `surveyor` from the platform-engineering checkout once. For each
range: a detached scratch worktree at the range's head
(`git worktree add --detach`), `TESTKIT_GO_JSON=<surveyor> task
tests:attest` run IN the worktree (binds the run to that worktree's own
HEAD commit via `git rev-parse HEAD` / `git status --porcelain`), then the
NEW `fathom ingest` and `fathom conserve --since <base> --json` run
FROM the worktree but against a scratch store under
`<worktree>/workspace/tmp/m.sqlite` (never the shared store, never the
sextant checkout at `/Volumes/m2ext/code/geoffmilleraz/sextant`).

One operational note: `go test`'s build cache lives on the external volume
(`GOCACHE=/Volumes/m2ext/tmp/cv3-build`), and the well-known EPERM wedge
(`~/.claude/guides/external-volume-eperm.md`) fired on both worktrees'
first `task tests:attest` (`go: creating work dir: ... operation not
permitted`, an empty `go-test.jsonl`, envelope `run: 0`). `tccutil reset
SystemPolicyRemovableVolumes com.mitchellh.ghostty` and a retry fixed it
both times, as documented; the SECOND run of each is the one this reading
uses (confirmed by the `run` totals below being nonzero and the recorded
`commit` matching the worktree's HEAD).

Because `conserve.Residue`'s own JSON `items[]` deliberately lists only
`unreached`/`unattested` functions (design §3.4: attested items are
counted, never listed — CV-63's own contract), and both ranges came back
100% attested, hand-checking needed the full population, not just
`items[]`. A throwaway diagnostic (`cmd/dumpresidue`, `cmd/dumpcallers` —
built from THIS checkout so it links the reading-set feature, never
compiled inside a worktree that predates it; not committed, deleted with
the worktrees) calls the same exported `conserve.NewInput` +
`conserve.ChangedFunctions` + `callgraph.BuildResolved`/`TestImpactDetail`
+ the same attestation rule `isAttestedBy` uses, and prints every changed
function's class, not only the unattested/unreached ones. Its per-range
totals (`changed`/`attested`/`unattested`/`unreached`) matched
`fathom conserve --json`'s own `residue` object exactly on both ranges —
cross-checking that the diagnostic reproduces the real tool rather than
diverging from it.

## Range 1 — CV-M6 dogfood (retire 18 unused analysis packages)

Base `c75b70e8` (2026-09-24 20:03 -07, "add retire-unused-analyzers.md"),
head `4957b916` (2026-09-24 20:10 -07, "flip retire-unused-analyzers.md
rows to [x] — verify proves them") — the commit
`docs/bench/readings/cv-m6-dogfood-2026-09-24.md` and this repository's own
`git log` name as the one that finished CV-M6's program (the deletion
commit `5068356f` plus the flip commit `4957b916`, both inside this range).

| metric | value |
|---|---|
| changed files (`git diff --name-only`) | **41** (3 modified, 2 modified-no-func-change, 36 deleted across 18 retired packages) |
| test run | 6965 run, 6952 passed, 0 failed, 13 skipped — bound to `4957b916`, dirty=false |
| changed Go functions (reading set's raw material) | 1 |
| attested / unattested / unreached | 1 / 0 / 0 |
| `items[]` (unattested+unreached) | 0 |
| unanalyzed files | 3 (`CHANGELOG.md`, `docs/plans/retire-unused-analyzers.md`, `docs/plans/review-verb.md`) |
| **reading set's files + unanalyzed** | **3** |
| fewer than changed files? | **yes — 3 < 41** |

The single changed function is `runLocalize`
(`internal/mcp/localize_tool.go:564`, `modified`) — a comment-only edit
inside its body (the "recency" hop's skip-note, updated to name that the
analyzer it used to point to, `internal/analysis/churn`, was itself
retired by this same program) and unrelated file-header doc-comment edits
to `internal/core/model.go` that touch no function body at all, so that
file contributes zero changed functions. The other 39 changed files are
either non-Go (3, listed above) or deletions (36, across the 18 retired
packages) — deletions are, correctly, outside the reading set's scope by
design (§3.4: "Deleted functions are not in the reading set"; the
resurrection and absence planes cover them, and `fathom conserve`'s
`resurrection`/`absence` planes both read `conserved`, zero findings, for
this range). The `integrity` plane (a different plane, not the reading
set) separately flagged one pre-existing guarded test,
`TestAnalyze_SortedDescending`, `deleted` with no declaration — real, but
outside CV-M12's scope; noted for completeness, not adjudicated here.

## Range 2 — CV wave 3 gap-closing

Base `5afeee42` (2026-09-24 20:21 -07), head `78e508d3` (2026-09-24 22:31
-07, "fix(compile): real-run WithLLM call sites reference the named rate
constants (O119)") — the exact range the task handed this reading.

| metric | value |
|---|---|
| changed files (`git diff --name-only`) | **47** (16 modified, 15 added, deletions: 0) |
| test run | 7000 run, 6987 passed, 0 failed, 13 skipped — bound to `78e508d3`, dirty=false |
| changed Go functions (reading set's raw material) | 31 |
| attested / unattested / unreached | 31 / 0 / 0 |
| `items[]` (unattested+unreached) | 0 |
| unanalyzed files | 21 (docs/vision/ADR/CHANGELOG/`.reviewkit/*.cue`/CODEOWNERS — every changed file that is not Go) |
| **reading set's files + unanalyzed** | **21** |
| fewer than changed files? | **yes — 21 < 47** |

All 31 changed functions span 13 non-test Go files: 3 brand-new files
(`cmd/fathom/modelprice.go` — 1 function; `internal/analysis/inventory/
apicompat.go` — 12 functions, all `added`, the CV-M4 apidiff-compatibility
machinery; `internal/mcp/review_integrity.go` — 3 functions) and 10
modified existing files (`cmd/fathom/compile.go` 2,
`cmd/fathom/servehttp.go` 1, `internal/analysis/inventory/inventory.go` 4,
`internal/conserve/input.go` 2, `internal/conserve/resurrection.go` 1,
`internal/mcp/review_tool.go` 2, `internal/scip/scip_import.go` 1,
`internal/semantic/compile.go` 1, `pkg/client/semantic.go` 1) —
1+12+3+2+1+4+2+1+2+1+1+1 = 31, exactly matching the report. One modified
file, `internal/core/claims.go`, contributes zero changed functions (its
only edit is a new struct field, `PriceKnown` — correctly out of the
reading set's scope, which covers function bodies only; the surface plane
covers struct fields, and reads `conserved` for this range). The
`integrity` plane separately flagged 4 `edited` (informational-only, never
a loss) guarded-test bodies in the same range — expected, since this range
edited test files for CV-40/CV-41/CV-M4/O116 alongside production code.

## Hand check (20 items, both ranges)

Every item below is `attested` — the only class either range produced (see
Caveats). "Genuinely changed" was checked against `git diff <base>..<head>
-- <file>`: for `added`, the file itself is new or the function's `func`
line does not appear at base; for `modified`, a diff hunk's context lines
name the function and show its body text differing. "Class right" was
checked by finding a real reaching `Test*` (via `cmd/dumpcallers`'s direct
resolved-caller walk, or by reading the test file) whose latest run passed
bound to the range's own head commit on a clean tree — true for every run
in both worktrees' envelopes (see the per-range tables).

| # | range | file:line | function | change | genuinely changed? | class right? |
|---|---|---|---|---|---|---|
| 1 | 1 | internal/mcp/localize_tool.go:564 | runLocalize | modified | yes — comment-only body edit, hunk confirmed | yes — direct dedicated tests `TestLocalize_*` (18 of them) reach and pass; see note below |
| 2 | 2 | cmd/fathom/compile.go:229 | compileViaStore | modified | yes — 3 hunks inside the function | yes |
| 3 | 2 | cmd/fathom/compile.go:660 | printCompileRunCore | modified | yes — hunk inside the function | yes |
| 4 | 2 | cmd/fathom/modelprice.go:53 | rateForModel | added | yes — new file, function is new | yes — `TestRateForModel_KnownDefaultCompileModel` etc. directly test it |
| 5 | 2 | cmd/fathom/servehttp.go:50 | runServeHTTP | modified | yes — hunk inside the function | yes |
| 6 | 2 | internal/analysis/inventory/apicompat.go:99 | typeDetailFor | added | yes — new file | yes — reached via `Diff`, direct tests below |
| 7 | 2 | internal/analysis/inventory/apicompat.go:116 | structFieldShapes | added | yes — new file | yes |
| 8 | 2 | internal/analysis/inventory/apicompat.go:190 | itemChanged | added | yes — new file | yes — `TestDiff_AdditiveStructFieldIsNotChanged` etc. exercise it directly |
| 9 | 2 | internal/analysis/inventory/apicompat.go:200 | typesCompatible | added | yes — new file | yes |
| 10 | 2 | internal/analysis/inventory/apicompat.go:264 | normalizeAnyType | added | yes — new file, hunk also touches its 2 call sites | yes — `TestDiff_AnyInterfaceEmptyIsNotChanged` |
| 11 | 2 | internal/analysis/inventory/inventory.go:175 | Diff | modified | yes — hunk rewrites its body (Item-keyed instead of Detail-keyed) | yes — 15 `TestDiff_*` tests reach and pass |
| 12 | 2 | internal/analysis/inventory/inventory.go:219 | extractGoAPI | modified | yes — hunk inside the function | yes |
| 13 | 2 | internal/analysis/inventory/inventory.go:654 | renderNode | modified | yes — hunk adds `e = normalizeAnyType(e)` | yes |
| 14 | 2 | internal/conserve/input.go:244 | NewInput | modified | yes — 2 hunks inside the function | yes |
| 15 | 2 | internal/conserve/input.go:297 | planPathRelativeTo | added | yes — new function, confirmed absent at base | yes |
| 16 | 2 | internal/conserve/resurrection.go:101 | Resurrected | modified | yes — hunk inside the function | yes |
| 17 | 2 | internal/mcp/review_integrity.go:36 | applyIntegrityDimension | added | yes — new file | yes — `TestBuildReviewInput_IntegrityDimensionWiredEndToEnd` |
| 18 | 2 | internal/mcp/review_tool.go:66 | registerReviewDiff | modified | yes — tool `Description` string literal changed | yes — `TestReviewDiff_IntegrityDescriptionMentionsIt` directly asserts the new text; see note below |
| 19 | 2 | internal/scip/scip_import.go:543 | fromIndex | modified | yes — hunk inside the function | yes |
| 20 | 2 | internal/semantic/compile.go:238 | Compiler.Compile | modified | yes — hunk inside the function | yes |

**Result: 20/20 genuinely changed, 20/20 correctly classed. Zero wrong
items.** No false `attested`, no missed `unattested`/`unreached` found in
the sample (there were none to miss — see Caveats).

**Note on items 1 and 18 (register-pattern MCP tool handlers).**
`cmd/dumpcallers` walked `runLocalize`'s DIRECT resolved callers: exactly
one, `registerLocalize` (`internal/mcp/localize_tool.go:909`), itself
called only by `registerTools` (`internal/mcp/tools.go:28`), itself called
only by `newServer` (`internal/mcp/server.go:184`). Because the actual
call to `runLocalize` sits inside a function-literal closure registered as
a value (`s.registerTool(desc, func(ctx, args) {...})`), the typed pass
attributes that call to the LEXICALLY ENCLOSING named function
(`registerLocalize`), not to whichever handler eventually invokes the
closure at runtime — so ANY test that merely constructs a server (which
unconditionally registers every tool, `runLocalize` included, whether or
not that test ever issues a `localize` request) statically "reaches"
`runLocalize` and `registerReviewDiff` over resolved edges. This is a real
and general limitation of static-call-graph test-impact (not unique to
fathom, not a violation of design §3.4's own stated contract — CV-61
defines "reach" as resolved-edge reachability, not runtime execution) —
for a function reached ONLY through a registration table like this, an
`attested` verdict can be satisfied by a test that never actually runs
that handler. In both concrete cases checked here it did not matter: 18
dedicated `TestLocalize_*` tests and `TestReviewDiff_
IntegrityDescriptionMentionsIt` are ALSO in the reach set, genuinely
exercise exactly what changed (a comment; a description string), and
pass — so the classification is correct in fact, not only by the letter
of the rule. But a future range that changes a registered handler's real
LOGIC, with no dedicated test for that logic, would read `attested` on
this same registration-chain technicality alone. That is a caveat about
`attested`'s strength for this one architectural pattern (a central tool
registry), not a defect this reading found.

## Caveats

- **Nothing to catch, on either range.** Both ranges are 100% attested —
  zero `unattested`, zero `unreached`, on 32 changed functions combined.
  Both ranges also passed `task validate`'s own `go test -race ./...` gate
  before being committed (this repository's own discipline), so a clean
  reading set on THESE two ranges is the expected result, not a surprise.
  CV-M12's hand check asks for "a mix of classes"; there was only one
  class to sample. This reading proves the reading set does not cry wolf
  on two real, disciplined ranges — it does NOT prove the reading set
  correctly flags a genuinely undertested change, because neither
  available range contained one. CV-M5's Blindness replay tested a
  DIFFERENT plane (resurrection) against seeded defects; nothing has yet
  tested the reading set itself against a seeded or historical
  undertested-function range. That is real, unmeasured risk this reading
  does not resolve — a natural next reading if the owner wants sensitivity
  evidence, not required by CV-M12's own literal bar.
- **Comment-only edits count as "changed."** `ChangedFunctions` compares
  EXACT body text (design §3.4, `exactHash`), not a normalized hash, so a
  pure comment edit (item 1: `runLocalize`; item 18's `Description` string
  is data, not a comment, but the same principle) is `modified` even
  though behavior is byte-identical. This is documented, intentional
  (conservative — it can only ADD items, never hide a real change), and
  matches the design doc's own stated rule; noted here only because it
  explains why a change with zero logic difference still shows up.
- **Register-pattern reach** (see the note above): `attested` for a
  function reached only via a central tool-registration table can be
  satisfied by any test that builds a server, not only one that exercises
  that specific tool. Confirmed harmless for the two instances checked
  here; not exhaustively checked for the other 30 items.
- **CV-51's trigger has not fired.** Both measured reading sets came back
  at 0 items (well under the 100-item threshold CV-51 names) — no signal
  yet on whether a caller-count ordering needs a model-assisted rank.
- Range 1's `4957b916` was chosen as "the commit that finished [CV-M6's]
  program" per `docs/bench/readings/cv-m6-dogfood-2026-09-24.md` (the flip
  to `[x]`, immediately after the deletion commit `5068356f`) — an
  operator judgment call the task handed to this reading, not a value
  named literally in the plan row.

## Reproduce

```sh
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod \
       GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp \
       GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOFLAGS=-mod=mod

# from this checkout (branch reading-set)
go build -o /tmp/rs-tools/fathom ./cmd/fathom

# from a platform-engineering checkout
( cd kits/test-kit && go build -o /tmp/rs-tools/surveyor ./cmd/surveyor )

for spec in "c75b70e8 4957b916" "5afeee42 78e508d3"; do
  set -- $spec; base=$1; head=$2
  wt=/tmp/rs-measure/$head
  git worktree add --detach "$wt" "$head"
  ( cd "$wt" && TESTKIT_GO_JSON=/tmp/rs-tools/surveyor task tests:attest )
  mkdir -p "$wt/workspace/tmp"
  /tmp/rs-tools/fathom ingest --service fathom --path "$wt" \
    --store "$wt/workspace/tmp/m.sqlite"
  ( cd "$wt" && /tmp/rs-tools/fathom conserve --since "$base" --json \
      --service fathom --store "$wt/workspace/tmp/m.sqlite" )
  git worktree remove --force "$wt"
done
```

On any `Operation not permitted` from `go`: `tccutil reset
SystemPolicyRemovableVolumes com.mitchellh.ghostty`, then retry the exact
same command.
