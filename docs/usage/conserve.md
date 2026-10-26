### `conserve` — the refactor-conservation report (ADR-044, CV-19)

```sh
fathom conserve --since <git-ref> --service <id> [--store DB] [--json] [--strict] [--strict-plane integrity] [--plan FILE ...] [--residue-max N] [--coverage-meta FILE]
```

Runs every registered `internal/conserve.Plane` (`surface`, `certificates`,
`governance`, plus `integrity`/`absence` once landed) over base (`--since
<ref>`) versus head (the working tree, uncommitted edits included) and
reports each plane's verdict: `conserved` (checked, nothing undeclared
lost), `losses` (at least one undeclared finding), or `unproven` (an
input the plane needed was missing — never `conserved` from a missing
input, I44). The `surface` plane's own findings are the capability
inventory's base→head delta (`internal/analysis/inventory`: exported Go
declarations, JSON wire tags, routes, env keys, proto RPCs, plus any
`[[surface.pattern]]`-configured kind, `docs/configuration.md`) — a
removed or changed `route:` key additionally names any OTHER known
service whose client code still calls it (`consumer_loss`). `certificates`
reports a `// certifies:` marker declared at base and gone at head, plus
any compiled certification whose test or reach is in the changed set and
whose state is `unproven`/`broken`/`unreachable`. `governance` reports an
obligations/adr-rules/spec-bindings/findings claim whose scope glob(s)
matched a file at base and match none at head — a deliberate exception
that reads the full base/head file-path list (`git ls-tree`/`git
ls-files`), never file content, since a claim's scope can be orphaned by
files nowhere near this particular diff.

A finding is excused (its `declared` field is `"prior"`) only when a plan
row's `retires:`/`surface_change:` key already named it BEFORE the change
range started — `--plan` (repeatable) names which plan files to read those
declarations from; unset, every `docs/plans/*.md` at the repo root
(non-recursive, the same set the pre-push gate itself walks) is read. A
declaration added inside the range excuses nothing (I45) — it shows up as
`declared: "self"` instead, exactly like `fathom verify --since` renders a
self-declared row.

One kind is a deliberate exception: a `resurrection`/`moved` finding (an
ordinary, undeclared rename — design §3.3's resurrection honesty) always
carries `declared: "prior"` on the wire even though nothing was ever
declared, because "prior" is the only value that excuses a finding from
`losses` and nothing needed excusing. The text renderer and `review_diff`
never print that raw value for a `moved` finding — both say "no
declaration needed" instead, so the line never claims a declaration was
made when its own Detail says the opposite; a caller reading the JSON
directly still sees `declared: "prior"`.

```json
{"schema":"fathom.conserve/v1","service":"fathom","base":"<sha>","head":"worktree",
 "planes":[{"plane":"surface","verdict":"losses","missing":[],
   "findings":[{"plane":"surface","kind":"removed","key":"api:foo.Old",
     "detail":"func()","base":"foo.go#L3","declared":"none",
     "file":"foo.go","line":3,
     "remedy":"add `retires: api:foo.Old` to the row that removed it, or restore it"}]}],
 "tiers":[{"language":"Go","tier":"typed"},{"language":"TypeScript","tier":"file-resolved"}]}
```

Every finding also carries `file`/`line` (both `omitempty`) — the
repo-relative location of the finding's SUBJECT, at HEAD when it still
exists there, else at BASE — for a review kit citation that needs a real
source anchor rather than the human-readable `base`/`head` strings above.
Empty when no plane could place the subject at a location cheaply (a
`governance` claim's own scope glob names no single file, for instance);
never invented.

`--json` emits the versioned `fathom.conserve/v1` document above (the
compatibility surface: field names and verdict/declared strings are
stable); the default is a short text table, one line per plane plus its
findings and missing inputs indented beneath, and a trailing `tiers:` line
naming the edge tier (`typed`/`scip`/`file-resolved`/`syntax`) behind every
language among the changed files. `--strict` exits 1 when any plane's
verdict is `losses` (2 is reserved for an operational failure — a bad
`--since` ref, an unreadable store; 0 otherwise, --strict included, when
no plane reports losses).

`--strict-plane integrity` (PR-9, ADR-057) is the one per-plane gate, the
one `lefthook.yml`'s pre-push `refactor-conserve` step runs: it checks the
range's **pins**, the tests a governing source names (an invariant's
enforcement column in `docs/intent/invariants.md`, a `tests/*.cue` manifest
entry, a compatibility-surface evidence line in `docs/intent/scope.md`, a
`// certifies:` marker, a `done needs:` segment of a guard note in
`.fathom/guard.cue`), with reach off, and prints one line per lost pin
after the report (on stderr under `--json`, so stdout stays the
`fathom.conserve/v1` document). The loss kinds are `deleted`, `renamed`,
`weakened`, `skipped`, `golden_changed` and `marker_moved`; `edited` is
never one, and a test moved unchanged to another changed file is not lost.

```text
pin lost (silent, blocks): deleted TestBackendForPath_DuckDBLoudlyRefused, named by invariant_enforcement docs/intent/invariants.md:23 and still named there at head, TestBackendForPath_DuckDBOpensAsSQLite is new in its place (internal/store/backend_test.go:114)
pin retired without a decision cite (blocks): deleted TestOpenPlatform_LoneDuckDBFileErrorsLoud, named at base by invariant_enforcement docs/intent/invariants.md:23; the source was edited and no longer names it, but no decision record or dated decision line added in the range names it
pin retired (review): deleted TestPinned, named at base by invariant_enforcement docs/intent/invariants.md:5; the source was edited and no longer names it; decision cite ADR.md:5
integrity pins: 1 silent loss(es), 1 uncited retirement(s), 1 cited retirement(s)
```

It exits 1 on a **silent** loss: the lost test is still named by its
source at head. A **retired** pin, whose naming source was edited in the
range and no longer names it, also exits 1 unless the range cites a
decision that names it (PR-12, ADR-057's K4 clause): a line added in the
range that names the test and either sits in a decision record (`ADR.md`,
`docs/adr/**`, `docs/intent/decisions/**`) or is a dated decision line (an
ISO date and `decision`, `decided`, `retire`, `retired` or `retires` on the
same line) in any changed file. A line under `docs/plans/**` never counts,
and a line only moved is not added. A cited retirement is printed for
review with its cite and exits 0. An input the check could not read exits
2, like any other operational failure; the pre-push step fails open on it.
It combines with `--strict` (either can exit 1).

#### Kubernetes concern deltas

```sh
fathom conserve --since BASE --service SERVICE --concern-deltas workspace/artifacts/concern-deltas.json
```

`--concern-deltas FILE` writes experimental `fathom.concern-deltas/v1` JSON for
Kubernetes manifests in the touched set. This schema remains experimental until
PLT-M4 and is not yet part of the compatibility surface. The touched set combines
the committed range with `in.ChangedFiles`, which includes uncommitted worktree
files (staged edits and untracked files included). Create the destination
directory first. Head
`K8sFact` values come from scoped reads of the service's store and require stored
`tool_kind` of `k8s` or `k8s-crd`. Base blobs from `git show BASE:<path>` pass
through ingest's extractor dispatch with base sibling-path context and the
connector's corpus exclusions and size cap, inside the CLI process without store
writes. Helm chart templates use `HelmTemplateYAML`, and JSON manifests use the
JSON extractor, so neither is compared as Kubernetes. Ignored paths, including
`testdata/`, `vendor/` and `.fathomignore` matches, and oversized files are skipped.
Base uses its committed project ignore and connector configuration with current
user/service exclusions. A missing service index is an operational error
(exit 2), as is an unwritable destination.

The file's fields are `schema`, `service`, `base` (resolved commit), `head` (HEAD
commit), `basis`, `deltas`, and `skipped`. Each delta carries:

- `workload`: Kubernetes `<kind>/<name>`, plus separate `namespace` and `container`.
- `key`: the extractor's fact key, including its container prefix when applicable.
- `fact_kind`: `K8sFact`, and `fact_version`: `v1` (the fact key/value contract,
  independent of Kubernetes `apiVersion`).
- `change`: `added`, `removed`, or `changed`, with `before` and `after` values.
- `base_anchor` and `head_anchor`: relative `file:line` locations at each side.
- `basis` and `head_commit`.

A missing side is `null`, with an empty anchor. Unchanged facts emit no delta.
Facts pair by workload and key across all touched manifests, so identical moves
and document splits emit no deltas. If a fact name appears in multiple files on
either side (for example kustomize patches), pairing also requires the same path.
A move plus edit retains each side's own file and line. A namespace change reads
as removed under the old namespace plus added under the new one.
Renaming a container removes its old fact keys and adds its new ones, and changes
the container-list fact. Only facts the extractor supports are compared. Env
values, arg values and probe exec commands are withheld by the extractor and
never reconstructed for this file. An env removal that emits no `K8sFact` emits
no delta.

`basis` carries `state`, `index_commit` and `head_commit`. Its state is `exact`
when the index commit equals HEAD, no compared file is dirty and each stored
fingerprint matches the epoch-salted hash of its HEAD blob. `differs` means the
commits differ (both are named) while those file checks still agree.
`uncommitted` means a compared file is dirty (including staged changes), its
fingerprint is absent or differs from HEAD, or a stored fingerprint exists for a
path absent at HEAD. This catches dirty-ingest-then-revert and extractor skew.
Uncommitted evidence takes precedence over commit skew. Values still come from the store in the latter two states. Consumers needing exact
commit evidence should use only `exact`.

Undecodable YAML, including undecodable Helm templates, emits no deltas for that
file. Ingest dispatch exclusions also emit no deltas. `skipped` names every
excluded touched `path`, its `commit` and `reason`. This includes
both commits if both fail decoding. Partially undecodable multi-document files
are skipped as a whole. Both lists are sorted. Deltas sort by workload, namespace,
key, base anchor, head anchor and change. Skips sort by path and commit. Repeated
runs over the same inputs produce identical bytes.

These deltas are information, with no direction rule or verdict. They appear in
`Report.ConcernDeltas` (`concern_deltas` under `--json`), beside `residue`, and never
affect `--strict`. They are not a registered plane and do not run in the daemon's
`review_diff` path. Replicas going from 3 to 1 is a reported change for the consumer
to judge.

#### The reading set (design §3.4, ADR-044 amended 2026-09-25, Q22)

`fathom conserve` prints a `reading set` section AFTER every plane's own
table: the changed functions in `base..head` whose behaviour no
commit-bound passing test vouches for — what a reviewer still has to open
once every plane has spoken. It is **not a plane**: it carries no verdict,
changes none of the planes' verdicts, and never affects `--strict`'s exit.

For each changed, non-test, non-generated `.go` file, every function
`added` (no function of that receiver-qualified name existed at base
anywhere in the same changed package directory) or `modified` (one did,
and the exact body text differs) is classed by walking the index's
resolved call edges back to `Test*` functions:

- `attested` — at least one reaching test's latest run passed, is bound to
  a commit, was made on a clean tree, and neither the test's file nor the
  changed function's file changed since that commit (the same O114 rule
  certificates use, applied here without a declared target);
- `unattested` — a test reaches it, but none has such a run;
- `unreached` — no test reaches it over resolved edges at all.

A moved-but-unchanged function (same body, different file in the same
package directory) is neither added nor modified, and a deleted function
is never in the reading set (the compiler proves it gone; the
resurrection/absence planes cover the rest). Items are ordered `unreached`
then `unattested`, each by resolved caller count descending then symbol —
`attested` functions are counted but never listed, since nothing there
needs a reviewer's attention. The list is capped (`--residue-max`,
default 200); the report names how many were cut. **No percentage, ratio
or threshold is ever computed (I43): counts and lists only.**

The reading set is `unproven` — classing nothing, only naming what was
missing — when the index is not at HEAD (the store's last-ingested commit
must equal git HEAD, working tree clean), the typed pass failed, or a
changed file's base content could not be read (I44). `changed` and
`unanalyzed` (every changed file that isn't Go, by path — the reading set
never extracts functions from any other language) are still reported in
that case: both are pure facts of the diff itself, computed without
touching the index, so a stale or partial store never taints them.

```json
{"changed":3,"attested":1,"unattested":1,"unreached":1,
 "items":[{"symbol":"Unreached","file":"widgets/extra.go","line":3,
   "change":"added","class":"unreached","tests":[],"callers":0},
  {"symbol":"Reached","file":"widgets/extra.go","line":9,
   "change":"modified","class":"unattested","tests":["TestReached"],"callers":1}],
 "truncated":0,"unanalyzed":[],"missing":[]}
```

Served two other places: `fathom.conserve/v1`'s additive `residue` object
(exactly the shape above), and `fathom review --since`'s `conservation`
dimension, which carries the same counts plus the first five `unreached`
items alongside its ordinary plane findings.

#### Execution evidence (CV-M13, design §3.5)

The reading set's original rule judged a function `attested` whenever a
passing, commit-bound test *statically reached* it — CV-M13
(`docs/bench/readings/cv-m13-reading-set-calibration-2026-09-25.md`)
measured that against real Go coverage and found it both too generous
(3.7% of `attested` functions never actually executed — branches no test
takes, a CLI verb switch, interface fan-out) and too strict (unreached
functions that DID execute, reached only through the standard library or a
runtime initializer, outside the first-party call graph).

`task tests:attest` now runs the suite ONCE as `go test -json
-coverpkg=./... -coverprofile=<results_dir>/coverage.out ./...` (the same
JSON output still feeds the envelope; `TESTKIT_COVER=0` turns the profile
off) and writes `<results_dir>/coverage.meta.json`:

```json
{"schema":"fathom.coverage/v1","commit":"<sha>","dirty":false,"race":false,
 "profile":"coverage.out","passed":true}
```

`fathom conserve` reads it from `--coverage-meta FILE`, or from `[tests]
results_dir/coverage.meta.json` when configured and neither is given. The
profile is used only when its meta names a commit, `dirty` is false and
`passed` is true (I44) — an unbound, dirty or failing-run profile is never
used, and the report names why (`coverage_unusable`). A profile whose file
keys map for some modules but not others (a second module's results left in
`results_dir`, a module renamed since the run) is still used, and the report
counts the unplaced keys with a sample (`coverage_unmapped`,
`coverage_unmapped_sample`): functions in those files have no execution
evidence and fall back to static reach. Per function, the
profile also counts only when neither the function's own file nor any
reaching test's file changed since that bound commit (the same O114 rule
certificates use) — a function whose profile has gone stale that way falls
back to static reach on its own, without disturbing any other item.

With a usable profile, a changed function's coverage blocks inside its own
span are mapped from the profile (`LoadCoverage`, module-path-aware,
duplicate blocks across a `-coverpkg` run's per-package sections merged by
MAXIMUM count) and its *changed blocks* — every block, for an added
function; only the blocks overlapping a `git diff -U0 base..head`-changed
line, for a modified one — decide the class purely from execution. A
function counts as modified only when its code tokens changed: a comment,
blank line or reflowed line inside the body (like a doc comment above it)
leaves it out of the reading set entirely.

- `attested` — every changed block executed (a modified function whose
  changed lines hold no block, such as a signature-only edit, clears);
- `partial` — the function executed (some block of it ran, changed or
  not), but some changed block never ran (`unexecuted_lines` names those
  ranges);
- `unexecuted` — no block of the function executed at all.

Without a usable profile — globally, or for one specific item — the
original static classes (`attested`/`unattested`/`unreached`) stand for
that item. Every item's `evidence` field names which rule classed it:
`"executed"` or `"static-reach"`. Items list every non-`attested` class,
in order `unexecuted`, `unreached`, `partial`, `unattested`, each by
resolved caller count then symbol — still counts and lists only (I43), no
percentage. `unanalyzed_functions` counts a changed function whose own
file the index never analyzed at all (vendor/, an ignore rule), so
`changed` always equals the sum of every class plus that count
(reconciliation, CV-71). `classed_by_execution` counts the changed
functions an execution-evidence profile classed (the attested ones, never
listed, included): `partial` and `unexecuted` exist only among them, so at
0 neither was measured and their 0 is not a count. Test-case identity: a case id carrying its own
package (`fathom tests ingest`'s join, `JoinCasesToSymbols`) disambiguates
a bare `Test*` name shared across packages, which used to leave both
unjoined (CV-M13's control measurement: 34 of 566 cases).

```json
{"schema":"fathom.coverage/v1", "...": "...",
 "residue":{"changed":2,"attested":0,"unattested":0,"unreached":0,
   "partial":1,"unexecuted":1,"unanalyzed_functions":0,"classed_by_execution":2,
   "items":[
     {"symbol":"Half","file":"widgets/extra.go","line":9,"change":"modified",
      "class":"partial","tests":[],"callers":0,"evidence":"executed",
      "unexecuted_lines":["12-14"]},
     {"symbol":"Dead","file":"widgets/extra.go","line":20,"change":"added",
      "class":"unexecuted","tests":[],"callers":0,"evidence":"executed"}],
   "truncated":0,"unanalyzed":[],"missing":[],"coverage_unusable":""}}
```

The decision port has no role (deterministic execution evidence, ADR-044
decision 8's "never safety").
