# Language breadth, wave lang8: foundation and deferred rows

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/language-breadth.lang8.md` proves it. Rows here are the
foundation and `-rest` rows wave lang8 files (`docs/plans/language-breadth.md` is edited only for a worker's own rows,
AP-14). Queue needs use the cross-plan form `language-breadth.lang8.md#LB-F<n>`.

## Foundation

- [x] A `cue-module` import strategy: a CUE import path names the files of the package it loads, by the cue binary's own rules: a path under the importing file's module (`cue.mod/module.cue`'s `module`, the nearest ancestor, its major version dropped) names the package directory there, whose files of the package (the path's last element, or its `:name` qualifier) and, when it has one, those of its ancestor directories up to the module root are the package; any other path is looked up in the module's `cue.mod/pkg`, `cue.mod/gen` and `cue.mod/usr` trees, which CUE reads as one; a standard-library path (no dot in its first element), a registry dependency outside the repository (the module cache is never read or fetched) and a `_`, `.`, `_test.cue` or `_tool.cue` file bind nothing, and none is a miss. The CUE extractor is a FileSpecExtractor and the harness resolves `cue` (`fileResolveLang`), so CUE's L3 is measured. Unblocks LB-Q-cue-rest's L3 {id: LB-F583, symbol: resolveCUEModule, package: internal/fusion, reachable_from: TestResolveCUEModuleBindsPackageFiles}
