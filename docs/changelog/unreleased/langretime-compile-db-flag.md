### Added

- `indexbench lang-retime -compile-db <subject>=<file>` (repeatable) installs a subject-relative companion database at `<subject>/build/compile_commands.json` while measuring and removes it on exit. It refuses an absolute `directory` or `file`, a different database already there, and a `build/` that git does not ignore.
