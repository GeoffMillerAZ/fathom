### Fixed

- Hook context dedupe is default-on for Claude Code only; every other harness emits in full until its tool events show `agent_id` and it provides a compaction reset. Full emits are recorded only after successful stdout delivery. Cwd-changed dedupe suppresses consecutive repeats only, and every wired pre-compact hook resets state. A 30-minute or 10-consecutive-suppression backstop re-emits full context after a missed reset. Invalid config emits in full, and a user's `[hooks] dedupe = false` wins over a repository's `true`.
