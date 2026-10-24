### Added

- fathom's own CUE configuration files now index as entities: each concern of `.fathom/concerns.cue` (those a comprehension generates included), each layer pack's `layer.cue` and each rule pack under `.fathom/rules/`. They read as CUE before. Stores ingested earlier re-extract these files on the next ingest.
