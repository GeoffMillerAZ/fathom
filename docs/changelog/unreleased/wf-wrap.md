### Added

- Declare command wrappers with `[shell] wrappers` in user or repository config so test-run hooks, readouts, gate preflight, Bash write detection and discovery can read their commands. Script wrappers support a declared working-directory argument and refuse unreadable scripts or nesting beyond two levels.
- Script wrappers without a declared working directory retain failure lines but leave test-run head/tree keys empty and gate preflight silent.
- Validate literal wrapper prefixes and distinct script/directory arguments, load declarations once per hook process and config path pair, and keep ordinary commands readable when config loading fails. Declarations require an explicit hook or transcript directory.
- Show each appended wrapper entry's source in `config --explain`.
