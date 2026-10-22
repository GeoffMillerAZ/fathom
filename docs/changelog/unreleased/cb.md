### Changed

- Edit-time obligations and the cwd-changed primer are no longer repeated within a session when unchanged; compaction or resume resets this. Set `[hooks] dedupe = false` to emit every injection.
