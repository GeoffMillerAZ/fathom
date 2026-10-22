### Changed

- On Claude Code, edit-time obligations and the directory-change primer are no longer repeated within a session when nothing changed: a session sees each unchanged injection once, compaction or a pre-compact hook resets that, and full context returns after 30 minutes or 10 skipped repeats. Other harnesses still receive every injection. Turn it off with `[hooks] dedupe = false`.
