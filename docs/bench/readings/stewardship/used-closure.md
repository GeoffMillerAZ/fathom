# SV-M3: the used closure of fathom's own mirrors

**Verdict in plain language.** `fathom report used` tells you exactly which part of a mirrored dependency
the product needs, and why, and it was right on both of fathom's mirrors. Its Go packages equal what `go
list -deps` says (the toolchain's own answer, matched exactly, on both mirrors and for two sets of entry
points), its CSS units equal the Tailwind source list `scripts/ui-css.sh` keeps by hand, and app-kit's mirror
pruned to the closure still builds (`task build`), passes `scripts/ui-css.sh --check-sources`, and passes the
160 console and app-kit tests of `cmd/fathom`. The product uses about a tenth of app-kit's bytes (1.38 MB of
15.8 MB) and a fifth of agentic-kit's. For Go alone it adds nothing over `go list -deps` except the reason
(entry point and first importer); what no single toolchain command gives is the join of Go imports, embedded
files and the CSS source list in one closure with a reason per unit. It pairs with `go list` and
`scripts/sync-vendor.sh` (a sync can read its JSON); it replaces neither. One gap: the closure does not know
the mirror's provenance stamp (`.vendor-commit`), which `ui-css.sh` requires, so the pruning script keeps it
and the licence file beside the closure. Keep `report used`.

SV-M3 verdict: KEEP

## The bars (vision 41 §4.5 and §7)

Fixed in the row before the run: Go units equal `go list -deps`; the CSS units equal `scripts/ui-css.sh`'s
declared list; app-kit's mirror pruned to the closure passes `task build` and `scripts/ui-css.sh
--check-sources`; units, files and bytes recorded before and after. A CUT is "Go units differ from `go list
-deps` after one fix round, or the pruned mirror does not build". No fix round was needed.

## Method ($0, scratch only)

`used-closure.sh` (beside this file) takes a detached worktree of this branch's HEAD under a scratch
directory, runs `fathom report used third_party/<mirror> --json` (all main packages, then `--main
cmd/fathom`), compares the Go units with `go list -deps` of the same mains filtered to the mirror's module
(`GOWORK=off GOFLAGS=-mod=readonly GOPROXY=off`), compares app-kit's `css-source` and `css-import` units with
the `third_party/app-kit/` entries of `ui-css.sh`'s `SOURCE_DECLARATIONS`, then prunes both mirrors with
`prune-to-closure.py` (a throwaway script reading the JSON's `used_files`; fathom never prunes, SV-X4) and
runs `task build`, `scripts/ui-css.sh --check-sources` and `go vet ./cmd/fathom` on the pruned tree. The
console and app-kit tests of `cmd/fathom` (`-run 'Console|Observatory|Styleguide|AppKit|Appkit'`) ran on the
pruned tree after it.

## Results

**Go units equal `go list -deps`, both mirrors, both entry sets.**

| Mirror | Entry points | Go units | `go list -deps` | Module packages |
|---|---|---:|---:|---:|
| app-kit | all 7 main packages | 22 | 22, equal | 85 |
| app-kit | `cmd/fathom` | 22 | 22, equal | 85 |
| agentic-kit | all 7 main packages | 25 | 25, equal | 203 |
| agentic-kit | `cmd/fathom` | 21 | 21, equal | 203 |

ADR-060 counted 22 of 93 app-kit packages and 19 of 242 agentic-kit packages on 2026-09-30. The module
package counts here leave out nested modules (app-kit's `data/design-assets` and `spikes/wails-gallery`,
agentic-kit's `kits/docextract/*` and the `testdata` fixture modules) and `testdata` trees, which are not
packages of the mirrored module; agentic-kit's pin has moved since (`cmd/fathom` now reaches 21).
`TestUsedUnitsMatchesListDepsOnThisRepository` keeps the `cmd/fathom` equality in the test suite.

**CSS units equal the declared list.** 19 units (18 `@source` package globs and the `@import` of
`appkit/ui/docs/prose.css`) equal the 19 `third_party/app-kit/` entries of `SOURCE_DECLARATIONS`. Every
declared `@source` directory is a package the Go closure reaches. Three reached `appkit/ui` packages have no
`@source`: `ui/internal/linetext`, `ui/internal/webref` and `ui/tokens`; none writes a class attribute (the
one `Class` match in `linetext.go` is Unicode classification), so the hand-kept list is complete.

**Pruned to the closure, the mirrors build.**

| Mirror | Units before → after | Go packages | Files before → after | Bytes before → after |
|---|---|---|---:|---:|
| app-kit | 445 → 50 | 85 → 22 | 1,806 → 215 | 15,775,329 → 1,381,175 (8.8%) |
| agentic-kit | 204 → 26 | 203 → 25 | 1,650 → 278 | 9,567,846 → 1,880,369 (19.7%) |

Units before are every Go package and every TS/JS module of the mirror (app-kit carries 332 TS/JS modules,
none reached) plus the declared roots; after are the used units: for app-kit 22 packages, 7 embedded files
(4 component scripts, the token schema and theme, the styleguide CSS), 18 CSS sources, 1 CSS import and the
two module files. Files after include `.vendor-commit` and `LICENSE.md`, which the script keeps beside the
closure.

- `task build` (with `FATHOM_CACHE_ROOT` unset so the build-cache wrapper keeps the warm module cache and
  nothing is fetched): exit 0, `bin/fathom` built and runs.
- `scripts/ui-css.sh --check-sources`: exit 0, "source closure valid". With `.vendor-commit` removed it
  refuses ("required source is missing: third_party/app-kit/.vendor-commit"): the provenance stamp is a
  declared input of the CSS build contract that no import, embed or CSS directive names, so the closure
  cannot see it.
- `go vet ./cmd/fathom` (test files compile too): exit 0.
- `go test ./cmd/fathom -run 'Console|Observatory|Styleguide|AppKit|Appkit'`: 160 passed, 0 failed.

## What it does not show

- Runtime reads: a file the product opens by path at run time (not embedded) is invisible to the closure.
  The console tests passing on the pruned tree is the evidence that none of the observatory's does.
- The full `ui-css.sh` build (not `--check-sources`) compiles app-kit's own `cmd/appkit` from the mirror;
  that tool is a build-time entry point, not a product one, and its closure is not in the product's.
- Pruning stays a script: the JSON's `used_files` is what a sync reads (SV-X4).
