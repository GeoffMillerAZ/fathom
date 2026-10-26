# Indented Sass candidate gate

The old statement that no indented Sass grammar exists is stale. The Zed SCSS extension pins
bajrangCoder/tree-sitter-sass at fb280c41b070657e4ff4d4e5e6eea6cb19efd9b8. Its LICENSE is MIT
and its current upstream HEAD is the same commit, with no release tags. The pinned repository
ships its generated C parser and scanner. No grammar source, calibration-subject source or
binary is copied into this change.

The queue's old subjects contain SCSS only, so they cannot measure indented Sass. The new Sass
shard and unchanged <=5% gate were committed in 13234fe9f before parsing either real-code
subject. Every tracked .sass file of bulma 0.9.4 is included, and every tracked .sass file under
vuetify v2.7.2's declared source subpath is included. No file was removed after a failure.

| Candidate native parse gate | Files | Files in error | Rate |
|---|---:|---:|---:|
| bulma 0.9.4 | 91 | 53 | 58.24% |
| vuetify v2.7.2, packages/vuetify/src | 128 | 70 | 54.69% |
| Pooled | 219 | 123 | 56.16% |

An independent first-party syntax cross-check, sass-parser 0.4.57's `sass.parse`, accepts 213
files. The candidate errors in 117 of those (54.93%). Thus the reference parser's six rejected
files do not account for the refusal. They remain in the original census. The receipt carries
both subject SHAs, all error-file paths and locations, the archive SHA256, ABI and runtime,
plus the reference parser's accepted/rejected counts. One zero-width node has an error flag
without a visible ERROR or missing child; its location and diagnostic limitation are recorded.

Reproduction: fetch the candidate archive at the recorded commit, read its LICENSE, compile
`src/parser.c` and `src/scanner.c` as a local shared library with `src` on the include path,
and load `tree_sitter_sass` into py-tree-sitter 0.25.2 (ABI 15). Parse each tracked scoped
.sass file with a two-second per-file timeout, counting a root error, missing node or timeout
as a faulting file. Independently parse those same files with sass-parser 0.4.57's indented
parser. Toolchains, clones, binaries and raw logs remain under workspace.

The result is a refused candidate and no support level. LB-Q-sass-rest changes from
deferred to refused under design section 14.2; controller board regeneration therefore
shows Sass as refused and moves its count from deferred to refused. No production WASM blob, dialect, dispatch rule, epoch or plan checkbox changes.
The generated board is left to the controller. Retry only when a newer
licensed pin passes the complete frozen corpus, then prove the production engine and L1-L3
against Dart Sass. No upstream patch, fork or issue proposal has been prepared. Measured diagnostic review
identifies multiline comma-continued selector lists in at least 65 of 123 error files.
Sibling combinators and interpolated selectors are additional observed failure shapes;
fixing comma continuation alone would still leave the candidate above the 5% gate.

## Named routes and reproduction (X-R1)

| Candidate or route | Version/pin | Concrete disposition |
|---|---|---|
| bajrangCoder/tree-sitter-sass, also the Zed SCSS extension's selected grammar | fb280c41b070657e4ff4d4e5e6eea6cb19efd9b8 | Latest checked HEAD, no release tags, MIT. Native complete-corpus gate fails 123/219 files, far above 5%. |
| P233/tree-sitter-scss | v1.0.1 / ee500b9c | SCSS braces grammar already used by Fathom, not an indented-Sass grammar. The queue's previous SCSS-only subjects could not validate indented syntax. |
| Dart Sass first-party indented parser | sass-parser 0.4.57 | Accepts 213/219 files; the candidate still fails 117 of those. This is independent refusal evidence, not a tree-sitter engine replacement. |
| Dart Sass compile / loadedUrls | 1.105.1 | Suitable later import-graph oracle, already used for SCSS. It cannot make a failed syntax grammar pass; no production dialect is admitted by this experiment. |
| Upstream grammar patch/fork or source pre-pass | candidate fb280c41 | No validated patch or semantics-preserving pre-pass is available in this change. It requires a new candidate and the unchanged complete-source gate; never drop failing source. |

The scripts and full setup commands now live in
`calibration/oracles/sass-grammar/README.md`, `gate.py`, and `reference.cjs`.
They check the frozen subject commits, clean trees and all 219 source files. No holdout was
used: both subjects are syntax-gate calibration, never an excellent/support claim.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / compiler | [Dart Sass 1.106.1](https://registry.npmjs.org/sass/latest) and [sass-parser 0.5.1](https://registry.npmjs.org/sass-parser/latest) are newer first-party parser/compile hits. No new grammar was parse-gated; a reference-parser version does not make fb280c41 pass. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-sass` search](https://github.com/search?q=scip-sass&type=repositories) yielded no measured indented-Sass indexer. |
| Language server | [VS Code CSS/SCSS service](https://github.com/microsoft/vscode-css-languageservice) does not document indented `.sass` grammar admission; no Sass-owned LSP was established by the bounded search. This negative search does not prove impossibility. |
| Registry AST | [sass_api 18.1.0](https://pub.dev/api/packages/sass_api) parses Sass/SCSS AST with spans. [`findDependencies`](https://pub.dev/documentation/sass_api/latest/sass_api/) supplies static URLs, not resolved file pairs; neither is a production Tree-sitter grammar. **LIVE UNMEASURED** as a future first-party oracle only. |
| Runtime/load tracing | [Sass compile/load API](https://sass-lang.com/documentation/js-api/) can report successful loads, not repair the candidate’s 123/219 source syntax errors. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.

X-R3 publication follow-up: the existing receipt is now a sanitized counts/hash
summary produced by committed `calibration/oracles/sass-grammar/publish.py`.
The 219/123 native and 213/117 reference-crosscheck measurements are unchanged.
Raw per-file diagnostics and compiler messages remain scratch artifacts; the
public receipt retains their hashes and aggregate diagnostic kinds. This is a
publication correction, not a new corpus reading or grammar result.
