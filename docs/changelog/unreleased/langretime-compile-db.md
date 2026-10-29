### Added

- `indexbench lang-retime -compile-db <subject>=<file>` (repeatable) installs a subject-relative companion database at `<subject>/build/compile_commands.json` while measuring and removes it on exit. It refuses an absolute `directory` or `file`, a different database already there, and a `build/` that git does not ignore.

### Changed

- `indexbench lang-oracle` records the compile databases an include-path dialect read as `compile_db` on each subject's tenancy: count, entries, and a root-independent fingerprint of per-file include roots, defines, header-root order and the effective target.
- `lang-retime` compares `compile_db` first, so the difference list opens with `compile_db: recorded <fp> (N database(s)) -> measured <fp|none> (M database(s))` and the "extraction changed" summary still follows. A reading without `compile_db` falls back to the database count in its include-path note.
