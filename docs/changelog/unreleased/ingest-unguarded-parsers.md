### Fixed

- A `.proto` file cut at an open bracket no longer hangs `fathom conserve`, the evidence report or the console index (the surface inventory handed the file to a parser that never returns for it), and a `.cue` file that opens with 2 MiB of `@a(` no longer ends the ingest in a stack overflow (the file resolver read its imports through the CUE parser without the nesting guard `Extract` has). Both now read as a file with no rpc or import.
