### `rules` — layer rules as tests, and as history (VB-14, VB-16, SR-47w)

```
fathom rules gen-test [--root DIR] [--out DIR]
fathom rules replay --since <ref|N> [--root DIR] [--rules FILE] [--format table|json|sarif] [--json]
```

`gen-test` compiles `.fathom/guard.cue`'s `layer_rules` to a Go test file. `replay` evaluates today's rules against every commit in a range (`ref..HEAD`, or the last N commits) and reports which commits would have failed the gate and which violations each one introduced versus inherited. `--format table` is the default; `--json` (or `--format json`) is the whole report. `--format sarif` emits SARIF 2.1.0 through the shared emitter (`guardsarif`, `verify.RenderSARIF`): one result per violation at the commit that introduced it (an inherited repeat is not a second finding), one rule per rule id, `error` or `warning` from the rule's severity, located at the offending package, the commit named in the message. `--format` wins over `--json`.
