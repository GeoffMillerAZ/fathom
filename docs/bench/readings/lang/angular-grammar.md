# Angular templates against the 5% ERROR-file gate (wave lf7, item angularastro)

**Verdict: no maintained Angular template grammar passes the gate, so nothing routes an Angular
template to an Angular dialect. On the two calibration subjects at their pins,
dlvandenberg/tree-sitter-angular v0.9.2 (its latest tag, and its default branch's head but for one
highlights commit) leaves an ERROR or MISSING node in 13 of the 72 templates the Angular compiler
reads (18.1%, three and a half times §14.2's 5% kill line): gothinkster/angular-realworld-example-app
1 of 10, angular/components' `src/material` package 12 of 62. Every one is valid Angular the
grammar lacks: a unary minus in a conditional branch (`? -1 : x`), a non-null assertion (`a()!`), a
structural directive's `let` microsyntax (`*rxLet="tags$; let tags"`) and a `@for` header's
trailing semicolon. LB-Q-angular stays open with its contract and LB-Q-angular-rest is written
`[-]` refused with this measurement; no dialect, blob or lock row is committed, and the templates
stay the html dialect's, as before.**

Measured 2026-10-11 at fathom 40aecf63 plus an uncommitted angular blob and a placeholder dialect.

## Grammar

dlvandenberg/tree-sitter-angular `v0.9.2`, commit `1446e647e50df96f6d4b1039f121b10b5c2b2976`
(tarball sha256 `5457e3f5d054287715c6acc6a46e24127976ea315da26c451dbccb0677d3fe20`). Not in the
catalogue (`indexbench lang-pin -id angular`: no pre-fill), pinned by hand. Licence MIT, read at the
pin from the top-level `LICENSE` (`indexbench lang-pin -tree`: `{"licence": "MIT", "licence_basis":
"LICENSE", "refused": false}`). The release ships `src/parser.c` (`LANGUAGE_VERSION 15`) and
`src/scanner.c`, so no generate step and no CLI pin were needed. Built with `build.sh` (zig 0.16.0,
wasm-opt 129) from a scratch lock row: module 250,355 bytes (sha256
`73529a584b23aa5e3fa2eadd4169161a903012eca0c92c685c7c9174a4532b28`), gzip 82,948 bytes. The
default branch's head on the day of measurement, `38a8014ed5452cd6b7cf1399c00177a1f5374256`, differs
from the tag by one commit to `queries/highlights.scm`, so it parses the same.

Candidates: every repository `gh search repos tree-sitter-angular` returned (2026-10-11) that is a
grammar of its own, maintained (a commit in the twelve months before the measurement, not
archived) and licensed. Only dlvandenberg's qualifies:

| Repository | Why not measured |
|---|---|
| memorileak/tree-sitter-angular | a copy of dlvandenberg's ("works with tree-sitter 0.26"), no licence file |
| BrokkAi/tree-sitter-ng | Java bindings that bundle dlvandenberg's grammar; archived |
| steelsojka/tree-sitter-angular, sykesm/tree-sitter-angular | GPL-3.0 (refused); no commit since 2022 and 2023 |
| captainko/tree-sitter-angular | no licence; no commit since 2021 |
| mclaughlinconnor/tree-sitter-angular-content | no commit since 2023 |

## Method

The file set is every template the Angular compiler reads in the subject: each `templateUrl` a
`.ts` file's decorator names, resolved against that file's directory (a scan of the subject's `.ts`
files; every named file exists). An inline `template:` string is no file and is not in the set.
`indexbench lang-oracle` with a placeholder `angular` dialect on the pinned blob, its manifest
claiming `**/*.html` and `-only` naming the set, reads the harness's own `files_with_error` (LB-F59:
an ERROR or MISSING node anywhere in the tree, the extractor's `tsHasError` test); a placeholder
query capturing `[(ERROR) (MISSING)]` named the line of each.

| Subject | Pin | Licence (read at the pin) | Templates | With an error (tree-sitter-angular v0.9.2) | tree-sitter-html v0.23.2, the lock's (beside) |
|---|---|---|---|---|---|
| gothinkster/angular-realworld-example-app | `build-50` `dd99ed2cf39c805d719f943c5d7061a5683d98a8` (Angular 21.2) | MIT (`LICENSE`) | 10 (`*.component.html`) | 1 (10.0%) | 0 |
| angular/components, package `src/material` (`@angular/material`) | `v22.2.2` `09bce199d1bee99e61b2f65c1cbf9649c234de6c` | MIT (`LICENSE`) | 62 (`<name>.html`) | 12 (19.4%) | 6 (9.7%) |
| **the gate's corpus** | | | **72** | **13 (18.1%)** | 6 (8.3%) |

Each file in error, with the first construct the grammar fails on:

| File | Line | Construct |
|---|---|---|
| realworld `src/app/features/article/pages/home/home.component.html` | 55 | `*rxLet="tags$; let tags"`: a structural directive's microsyntax with `let` |
| `src/material/button-toggle/button-toggle.html` | 5 | `disabled && !disabledInteractive ? -1 : tabIndex`: a unary minus in a conditional branch |
| `src/material/checkbox/checkbox.html` | 28 | the same |
| `src/material/datepicker/calendar-body.html` | 53 | `? 0 : -1` |
| `src/material/datepicker/datepicker-toggle.html` | 7 | `? -1 : tabIndex` |
| `src/material/datepicker/month-view.html` | 16 | `[todayValue]="_todayDate()!"`: a non-null assertion |
| `src/material/datepicker/multi-year-view.html` | 8 | a non-null assertion |
| `src/material/datepicker/year-view.html` | 8 | a non-null assertion |
| `src/material/paginator/paginator.html` | 11 | `[appearance]="_formFieldAppearance!"`, then `? -1 : null` |
| `src/material/slide-toggle/slide-toggle.html` | 11 | `? -1 : tabIndex` |
| `src/material/stepper/stepper.html` | 83 | `? 0 : -1` |
| `src/material/tabs/tab-group.html` | 70 | `@for (tab of _tabs; track tab;)`: a trailing semicolon, then non-null assertions |
| `src/material/timepicker/timepicker-toggle.html` | 8 | `? -1 : tabIndex` |

A minimal template holding only `[attr.aria-label]`, `[attr.aria-pressed]="!x ? a : null"`, a
trailing comma in an object literal or `$index + 1` parses clean; `[x]="a()!"` alone does not.

## What reads Angular templates today

The html dialect, by the `.html` extension, on tree-sitter-html: 6 of the same 72 templates in error
(8.3%; `src/material` 6 of 62, realworld 0 of 10), on Angular's bindings and `@`-blocks, which HTML's
grammar reads as attributes and text. That is html's reading of foreign syntax, recorded here and
not graded: no Angular construct (a selector's use, a pipe) is extracted from a template by any
dialect.

## The trigger

Re-plan when a tree-sitter-angular commit or release parses both subjects at these pins with at most
5% of their templates in error: re-measure with this method, then build the template dialect
(design §1.8's angularastro paragraph names the dispatch and the pair unit it would need).
