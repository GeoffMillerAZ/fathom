### `research` — the research flywheel

```sh
fathom research report [--since 168h] [--service S] [--root .] [--out DIR] [--call-log P] [--hooks-log P] [--bench-dir D] [--no-write] [--json]
fathom research bundle [--since 168h] [--service S] [--root .] [--out DIR] [--dry-run] [--include-retros] ...
```

`report` aggregates local evidence (call log, hooks log, context-bench
results, observations) into `workspace/artifacts/research/report.{md,json}`;
`bundle` renders the sanitized, shareable summary (schema
`schema/research-bundle/`), and sharing it stays a human verb.
`--include-retros` (IR-5, opt-in) adds a counts-only `retros` group built from
the retro corpus under the state directory's `artifacts/retros/`. It carries
channel and outcome cells over closed labels, and never a file, a session or
a verdict. Without the flag, or with no corpus, the group is absent, never
zero. Design: `docs/design/research-flywheel.md`, `docs/design/assess-lanes.md` §5.
