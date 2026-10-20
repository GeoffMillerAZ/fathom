### Fixed

- `search` with a directory in `path` (`src`, `packages/app`) found nothing, and with a glob (`packages/app/**`) it returned hits from gitignored build caches. The leading directory is now the search root, so ignore files apply under it, and the rest of the glob filters the results.
- `symbol_context` paired a definition from the index with a snippet read from the working tree, so after an edit it showed another function's code. When the file changed since the index, the snippet is withheld with `snippet_withheld: file changed since the index (<commit>)`; `read_source` serves the current text.
- A plan row anchored at a bare symbol that two packages define passed `symbol_present` and then failed `test_reaches` as untested. It now fails `symbol_present` with `ambiguous: N definitions (...); add package:`, and `test_reaches` and `edge_wired` read n/a.
