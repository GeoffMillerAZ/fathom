### Added

- Tech packs are discovered offline from the repository's own Go and CUE pins (a local `replace`, `vendor/`, or the module cache at the pinned version; never `go.work`, never fetched) and feed generated-file checks and layer loading: `fathom review` judges a declared generated output by its declaration before the same-directory heuristic, and a pinned pack's `layers/` join the layer union between the embedded layers and the project's.
