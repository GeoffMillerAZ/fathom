# Retire unused analyzers — CV-M6 (the dogfood refactor program)

CV-M6 (`docs/plans/refactor-conservation.md`): "retiring this repository's
unused analysis packages runs as a refactor program and finishes with every
row proven and a clean conserve report." Eighteen `internal/analysis/*`
packages have zero non-test importers anywhere in this module (a `go list
-json ./...` import scan, cross-checked against every `- [ ]`/`- [x]` row in
every `docs/plans/*.md` and every non-`internal/analysis/*` `.go` file for a
literal path/name reference) and no configured `.fathom/config.toml`
`[[surface.pattern]]` glob reaches `internal/analysis/*` source, so none of
them contribute `api:`/`json:`/`route:`/`env:`/`rpc:`/`cli:`/`tool:`/`verb:`
capability-inventory keys either — deleting them is a pure absence proof,
never a surface-plane loss. Each row below is `bin/fathom refactor plan
<import path>`'s own unedited output for that target: the tool found zero
importing packages for every one of the eighteen, so each plan is exactly
its own final `gone: import` row (no per-importer ratchet rows were
generated — there is nothing importing any of these today).

**Two candidates from the same zero-importer scan are excluded from this
program — both fail the check the same "zero non-test importers" search
implies and neither is safe to retire this wave:**

- `internal/analysis/migrationtargets` — its `Find` symbol is claimed by an
  already-`[x]` row in `docs/plans/pulled-tools-excellence.md` (`id: P1-6`).
  Deleting the package would fail that plan's own gate (`symbol_present`)
  without touching that plan in the same change, which is out of this
  program's scope.
- `internal/analysis/packagedeps` — has zero Go importers, but
  `internal/scenariobench/guardsfathom_probe_test.go`
  (`TestGuardsI1ConsumersDoubleRunDetectorIsNameless`) reads and rewrites
  `internal/analysis/packagedeps/packagedeps_test.go`'s literal committed
  text as a fixture for a guard-corpus regression probe (`git archive HEAD`
  of the whole repository, then a specific anchor string match against that
  file). A pure import/symbol scan cannot see this dependency — it is a
  textual fixture reference, not a Go import — so this program leaves the
  package in place. See `docs/bench/readings/cv-m6-dogfood-2026-09-24.md`
  for the finding.

Both exclusions are themselves evidence for CV-M6's own kill criterion: an
import scan (or a `safe_delete_check`-shaped store query) proves CODE is
unused; it does not know a plan file claims a symbol, or that a test
elsewhere depends on a package's file existing with specific content. A
refactor program built only on the import scan would have broken one gated
plan and one passing test.

Baseline: b5540cb1c31446828e6275b8d61ea2bb94da567f

## Absence rows (one per retired package)

- [x] github.com/geoffmilleraz/fathom/internal/analysis/antipattern is imported nowhere at head {id: retire-antipattern-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/antipattern}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/apichangelog is imported nowhere at head {id: retire-apichangelog-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/apichangelog}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/churn is imported nowhere at head {id: retire-churn-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/churn}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/cognitive is imported nowhere at head {id: retire-cognitive-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/cognitive}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/crossrepocycles is imported nowhere at head {id: retire-crossrepocycles-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/crossrepocycles}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/dataflow is imported nowhere at head {id: retire-dataflow-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/dataflow}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/deprecation is imported nowhere at head {id: retire-deprecation-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/deprecation}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/doccoverage is imported nowhere at head {id: retire-doccoverage-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/doccoverage}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/endpoints is imported nowhere at head {id: retire-endpoints-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/endpoints}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/erraudit is imported nowhere at head {id: retire-erraudit-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/erraudit}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/fieldusage is imported nowhere at head {id: retire-fieldusage-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/fieldusage}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/ineffassign is imported nowhere at head {id: retire-ineffassign-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/ineffassign}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/license is imported nowhere at head {id: retire-license-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/license}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/markers is imported nowhere at head {id: retire-markers-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/markers}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/naming is imported nowhere at head {id: retire-naming-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/naming}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/schemaexport is imported nowhere at head {id: retire-schemaexport-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/schemaexport}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/sqlmap is imported nowhere at head {id: retire-sqlmap-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/sqlmap}
- [x] github.com/geoffmilleraz/fathom/internal/analysis/structtags is imported nowhere at head {id: retire-structtags-gone, gone: import github.com/geoffmilleraz/fathom/internal/analysis/structtags}

> Run `fathom conserve --since b5540cb1c31446828e6275b8d61ea2bb94da567f --strict` before merging this refactor.
