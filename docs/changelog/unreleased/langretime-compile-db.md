### Changed

- `indexbench lang-oracle` records the compile databases an include-path dialect read as `compile_db` (count, entries, root-independent fingerprint) on each subject's tenancy. `lang-retime` compares it first and refuses with `compile_db: recorded <fp> -> measured <fp|none>` instead of a misleading "extraction changed"; a legacy reading falls back to the database count in its include-path note.
- `lang-retime -compile-db <file>` installs a companion database (paths relative to the subject root) at `<subject>/build/compile_commands.json` before measuring, and refuses when a different one is already there.
