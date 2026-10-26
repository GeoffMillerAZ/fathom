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

The result is a refused candidate and no support level. No production WASM blob, dialect,
dispatch rule, generated board, epoch or plan checkbox changes. Retry only when a newer
licensed pin passes the complete frozen corpus, then prove the production engine and L1-L3
against Dart Sass. An upstream patch or fork remains a prepared proposal for the maintainer.
