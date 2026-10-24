# Pairing fathom with RTK

Fathom recognises the direct `rtk <command> [args]` prefix. For example,
`rtk git status`, `rtk go test ./...`, `env X=1 rtk go test ./...` and
`rtk test go test ./...` expose the underlying command to claim checks,
gate preflight and tests-to-run classification. The `test`, `err` and
`summary` subcommands wrap another command. Literal global flags (`-v`,
repeated `v`, `--verbose` and `--skip-env`) and the `--` option separator
are supported.

Bare `rtk`, help/version flags and RTK's own tools and controls remain RTK
commands. These include `rtk read`, `rtk ls`, `rtk env -f AWS`, `rtk gain`,
`rtk config` and `rtk rewrite`. Unknown prefix options and dynamic command
names are not peeled. Other first words are directly wrapped commands.

The readout hook stands down for an RTK-prefixed run in observe, lean and
rich modes. Its test-run tier still records declared test commands and
emits the same failure context as when readouts are off, even outside the
readout roots. Rich mode keeps its test delta line for these runs. Only readout reshaping and enrichment stand down.
This exclusion applies to every recognised RTK-prefixed command family:
Git, Go, file listings and others. The same commands without RTK remain
eligible for their normal readouts. A compound command containing an
RTK-prefixed unit also keeps its entire output unchanged.

Fathom never installs RTK, reads or writes its configuration, imports its
filters, or ships RTK configuration. Recognition uses only the parsed
command prefix; it does not infer coverage from RTK settings.

When readouts are enabled and the command is within the configured roots,
the existing hook log records the exclusion on channel `readout` with kind
`raw`. Its metadata includes `reason: "rtk"`, `raw: "true"` and
`emitted: "false"`; no readout rule is selected. Inspect the hook log selected
by `FATHOM_HOOKS_LOG` (the normal default is fathom's `hooks.jsonl`).
