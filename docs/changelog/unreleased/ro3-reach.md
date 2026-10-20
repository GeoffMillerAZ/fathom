### Added

- `[hooks] tests_to_run` (off by default): after an Edit, Write or MultiEdit of a Go file, the edit hook adds one line naming at most 5 tests that reach the file, `and N more`, the `go test -run '^(A|B)$' ./pkg` recipe as text fathom never runs, and whether this session ran them at the current tree; a daemon that misses the line's 300 ms deadline reads `tests: not computed (deadline)`.
- Readouts rich mode asks the daemon, under its own deadline inside the hook's 1 s ceiling, for the definition of a compile error's identifier the working tree did not change and for the tests that reach the changed files (their own line, with the `go test` recipe); a miss ends the compile line with `index: not consulted (deadline)`.
