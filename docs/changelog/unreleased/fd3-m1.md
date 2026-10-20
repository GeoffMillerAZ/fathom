### Added

- `layer_rules` accepts kind `call_direction` (`from` and `to` package-name globs over call edges, `allow` a bool): code review and `arch_conformance` check it with the same engine as `rules/archconformance.cue`, which is still read. Doctor's `project` row prints the legacy pack's equivalent `layer_rules` entries once.
- The `[hooks] test_run`, `[hooks] pin_check` and `[guards] enforce` switches surface as the capability ids `test-run-hook`, `pin-check` and `guard-enforce`, valued by the same booleans. `capabilities.cue` wins on conflict and doctor names the conflict; `daemon.auto_refresh` stays user-only and is not surfaced.
