### Added

- `fathom culprit <good>..<bad>` ranks commits by failing-test reach, blast radius and recency and prints bisect commands for the operator without running tests or changing the checkout. It counts edits to failing tests, labels merges without charging inherited changes, and names missing evidence: commits with missing evidence rank below commits that reach a failing test and above indexed non-reaching ones. Output identifies the store and indexed commit, names unresolved tests once, and suggests testing the candidate's parent to confirm the boundary.
