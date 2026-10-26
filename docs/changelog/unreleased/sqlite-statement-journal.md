### Fixed

- Large call-index updates no longer abort an observation batch with a false SQLite "database or disk is full" error. Statement journals spill using a write size supported by the SQLite driver, preserving the batch's other files as well.
