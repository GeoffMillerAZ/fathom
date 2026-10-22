### Added

- Declare command wrappers with `[shell] wrappers` in user or repository config so test-run hooks, readouts, gate preflight, Bash write detection and discovery can read their commands. Script wrappers support a declared working-directory argument and refuse unreadable scripts or nesting beyond two levels.
