### Added

- Correctness hooks accept `"observe"` to compute and log their advice without injecting it. Init and reconcile register the required hooks in this mode.

- Observe hooks log `dropped=budget` when their computed line would exceed the output cap. Prompt definitions log their channel and line, missed or silent outcome; tests-to-run uses the existing `not indexed` miss spelling.
- Test deltas compare against the last decided, complete run of the same command shape and omit the delta clause when no comparison is possible. Compile readouts exclude formatter, dead-code, linter, and headerless module/embed diagnostics while retaining compiler errors with cut headers.
- Unindexed roots and unknown test-run verdicts stay quiet when no decided verdict is known. Claim checks retain a decided verdict at the same HEAD and tree since the last edit when a piped rerun is unknown, and report a missing command even when another command ran. Test-run lines omit negative reach claims when the index has not answered.
