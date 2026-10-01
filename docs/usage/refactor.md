### `refactor` — refactors as programs (CV-33/CV-34, ADR-044 §4)

```sh
fathom refactor plan <symbol|package|import> [--out FILE] [--service <id>] [--store DB]
fathom refactor status --plan FILE [--json]
```

`refactor plan` writes a gate-ready, grammar-v2 plan for retiring ONE
target — a symbol name, an import path, or a repo-relative package
directory (converted to its import path via `go.mod`'s own `module` line).
The target may be written before, after, or between the flags; unlike the
rest of this CLI (`fathom receipt verify`'s own doc note above: Go's `flag`
package normally stops at the first bare argument), this command
recognizes its own flag names wherever they fall so the documented
`<target> [--out FILE] …` order works exactly as written. Store-direct,
reusing the same `internal/analysis/callgraph` queries `rename_impact`/
`safe_delete_check` already answer with for a resolved symbol, and a plain
Go import scan (`go/parser`, `ImportsOnly`) for an import/package target.
The generated plan carries, in order:

1. a `Baseline: <HEAD sha>` header line (`docs/plans/README.md`'s
   plan-level pin);
2. one **ratcheted `gone: pattern` row per package that still references
   the target today** — grouped from typed, resolved caller edges for a
   symbol target, or from files that still import the path for an
   import/package target — each `max:` seeded at today's own lexical
   match count scoped to that package via `paths:` (so the row already
   passes once `fathom verify --store` checks it — `cliEvidence`'s
   `GoneEvidence` adapter);
3. a **`certified_by:` row per live-certified test** `TestImpact` says
   reaches the target (symbol targets only), so the refactor cannot
   silently break a certified behaviour while it proceeds;
4. the **final, unscoped absence row** — `gone: callers <Symbol>
   {package: <its own package>}` for a symbol target, `gone: import
   <path>` for an import/package target — proving the WHOLE corpus, not
   just the packages seen today;
5. a closing instruction line naming `fathom conserve --since <baseline>
   --strict`.

A ratchet's `max:` must never come from a partial read: when the corpus
cannot be fully read (a locked directory, a fetch error), `refactor plan`
refuses outright and names what could not be read, rather than seeding
`max: 0` for a package its own typed call graph may already show has live
references today. It also refuses on a DIRTY working tree: `max:` is
seeded from live disk while `Baseline:` is the last commit, so an
uncommitted edit would seed a `max:` that disagrees with what `refactor
status` later reads for that very same baseline sha (`git show`) —
commit or stash first.

`refactor status` reads one such plan's ratchet rows (any `gone: pattern`
row with `max:` set — hand-written or `refactor plan`-generated) and
reports, per row, its live count against `max:`, its count AT the plan's
own `Baseline:` commit (`git show`/`git ls-tree`, never a second ingest —
P19: cost scales with the row's own `paths:`, not the repo), and **drift**:
`max(0, current-baseline)`, references that landed on the branch since the
baseline. A plan with no `Baseline:` header is refused by name, never
silently reported as zero rows.

```json
{"schema":"fathom.refactor.status/v1","plan":"docs/plans/retire-old.md",
 "baseline":"<sha>",
 "ratchets":[{"id":"old-g1","pattern":"\\bOldFunc\\b","paths":["internal/caller/"],
   "max":3,"current":4,"baseline":3,"drift":1,"over_max":true}]}
```

`refactor status --migration "<kind> <from> -> <to>" [--store DB] [--services
a,b,c] [--conserve-since <ref>] [--json]` (CX-13, `docs/design/change-and-compliance.md` §4) is a
second, separate mode: the FLEET-WIDE migration view for a declaration in
`ParseMigration`'s own five-kind form (`symbol|import|route|config|pkg`),
across every service the store knows (or the ones `--services` names).
Store-direct like `refactor plan`: for each service, one at a time, it
scans that service's own sources for occurrences of `from` (remaining) and
`to` (done) — lexically via `internal/analysis/references`, upgraded to
`typed` in any file `internal/analysis/callgraph` proves has a resolved
caller of that symbol — and groups the result per repository
(`review.MigrationSites`, CX-10). A service that cannot be read degrades
into a named `degraded` entry rather than aborting the whole call. Purely
read-only: it computes and reports, it never rewrites a site (the Moderne
non-goal, refused as CX-X4).

Each remaining site names its **blocker** beside it (CX-11w,
`review.MigrationBlockers`): `generated` when its file has a
`// Code generated ... DO NOT EDIT.` header (or another language's marker) or the
repository's root `.gitattributes` marks it `linguist-generated` (nested
`.gitattributes` are not read), and `governed` — citing the rule id, with a warn
rule shown as `<id> (warn)` — when a layer rule in the repository's
`.fathom/guard.cue` forbids the file's package the migration's `to`: an
`import_direction` rule with the file in `from` and the `to` in its `to`, or a
`boundary` rule with the file off its `allow` list and the `to` in `guarded`. The
`to` is a directory: an import migration's as written or relative to the go.mod
module, a symbol migration's the directory of each indexed declaration of that
name; a route, config or pkg migration names none, so only generated code blocks
it. A governed file that is also generated reports the rule.

Each repository with a site or a `gone:` row also prints its **burn-down and
completion verdict** (CX-12w, `review.MigrationBurnDown`). The rows are the
repository's own `docs/plans/*.md` rows whose `gone:` contract names `from`
(`callers` or `pattern` for a symbol, `import` or `pattern` for an import,
`module` or `pattern` for a pkg, `pattern` otherwise), each proven when the
verify ledger's newest reading of it says so and unproven when it says otherwise or
never read it. The series is those rows' verdicts over time from the same ledger
(`console.ProofHistory`): at each reading where one changed, how many are claimed
and how many proven. Only readings recorded at a read repository's own root count
(PE-24): a push clone's or a scratch clone's are ignored, and `ledger_ignored` (text:
`verify ledger: …`) names how many. A repository is complete only when every row is proven and
conservation is established with no unexcused loss: `--conserve-since <ref>` runs
`fathom conserve --since <ref>` in each such repository, and without it none is
complete — the reason says conservation was not run, never that it passed.

```json
{"schema":"fathom.refactor.status.migration/v1",
 "migration":{"kind":"symbol","from":"OldFunc","to":"NewFunc"},
 "conserve_since":"origin/main",
 "repositories":[{"repository":"svc-a",
   "remaining":[{"file":"caller/user.go","line":5,"tier":"typed",
     "blocker":{"reason":"governed","rule":"caller-not-newpkg"}}],
   "done":[],
   "burn_down":{"rows":[{"plan":"docs/plans/mig.md","id":"MG-1","verdict":"proven"}],
     "series":[{"at":"2026-09-22T10:00:00Z","claimed":1,"proven":1,"remaining":0}],
     "complete":false,"reason":"…","conservation":"not run"}}]}
```

---
