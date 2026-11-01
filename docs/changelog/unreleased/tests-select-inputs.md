### Added

- `fathom tests select --inputs <dir>` selects the tests seen reading a changed file, from Go's own testlog observed per package and per test (`task test:inputs` in this repository), so an edit to a plan, changelog or reading selects the test that globs it; a package observed only whole runs whole (`"whole": true`). `task test:affected` runs the selection locally, and a failing pre-push suite prints how many of its failing tests the selection had named (ADR-090, TQ-31 to TQ-37).
