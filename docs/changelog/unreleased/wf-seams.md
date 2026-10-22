### Fixed

- Correctness hooks accept `"observe"` to compute and log their advice without injecting it. Init and reconcile register the required hooks in this mode.

- Test deltas do not label failures new against unknown, piped or truncated baselines. Compile readouts exclude formatter, dead-code, and headerless module/embed diagnostics while retaining compiler errors with cut headers.
- Unindexed roots and unknown test-run verdicts stay quiet. Test-run lines omit negative reach claims when the index has not answered, and claim checks count piped runs before reporting a missing run.
