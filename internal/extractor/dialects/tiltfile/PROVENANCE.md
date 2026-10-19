# tiltfile — provenance

LB-Q-tiltfile, design `docs/design/language-breadth.md` §13. A tags-engine tool format, new (no incumbent), on the
Starlark blob the bazel dialect pinned.

- Grammar: `tree-sitter-grammars/tree-sitter-starlark` v1.3.0 (MIT), pinned and built once for both dialects
  (`internal/extractor/tsengine/grammars/starlark.wasm.gz`, see `../bazel/PROVENANCE.md`); a Tiltfile adds no embedded bytes.
- `tags.scm`: written in this repository. A Tiltfile is a script, so a `def` is a function and each Tilt API call that
  declares an entity names it by its literal string: `local_resource`, `k8s_resource` and `dc_resource` (a resource,
  positional or `name =`), `docker_build` and `custom_build` (an image), `k8s_yaml("file")` (a manifest) and
  `config.define_*("name")` (a setting). An entity captures its name literal alone, not the call, so the call stays
  owned by the file's module symbol (`module_symbol`), as a call in a def is the def's. Every call (a plain name or an
  object's method, `config.parse`) is a reference by its last name.
- Kinds: `TiltFunction`, `TiltResource`, `TiltImage`, `TiltManifest`, `TiltSetting`, `TiltModule`. `Tiltfile` is classified
  (connector.go) as language `tiltfile`, typed-edge level `syntax`. No import is read: `load()` of `ext://` extensions
  names a repository outside the checkout, and a relative `load` or `include` needs a strategy no oracle grades.
- Oracle: tier C, `calibration/oracles/ts-tags` with `starlark-tags.scm` (O1349), subjects in
  `calibration/corpus.d/tiltfile.yaml`. Measured 2026-10-19, `docs/bench/readings/lang/tiltfile.json`.
