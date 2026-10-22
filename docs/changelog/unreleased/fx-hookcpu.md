### Fixed
- `fathom doctor --brief` (the Setup hook) skips the readouts row, which never fails: it no longer loads and fixture-tests every readout rule inside the hook deadline, and can no longer report that row as a timeout. `fathom doctor` still runs it.
