### `rules` — layer rules as tests, and as history (VB-14, VB-16, SR-47w)

```
fathom rules gen-test [--root DIR] [--out DIR]
fathom rules replay --since <ref|N> [--root DIR] [--rules FILE] [--format table|json|sarif] [--json]
fathom rules fixtures <pack dir> [--json]
```

`gen-test` compiles `.fathom/guard.cue`'s `layer_rules` to a Go test file. `replay` evaluates today's rules against every commit in a range (`ref..HEAD`, or the last N commits) and reports which commits would have failed the gate and which violations each one introduced versus inherited. `--format table` is the default; `--json` (or `--format json`) is the whole report. `--format sarif` emits SARIF 2.1.0 through the shared emitter (`guardsarif`, `verify.RenderSARIFRun`, with the run's provenance naming the range's end commit: see [verify](verify.md#sarif-provenance-and-fingerprints)): one result per violation at the commit that introduced it (an inherited repeat is not a second finding), one rule per rule id, `error` or `warning` from the rule's severity, located at the offending package, the commit named in the message. `--format` wins over `--json`.

`fixtures` reads a tech pack directory (`pack.cue` plus `fixtures/*.cue`) as pure data and prints a per-section reading: guard scope positives and negatives (`guards: [{guard: 0, match: [...], nomatch: [...]}]`), generated pairs (`generated: [{output, source: [...]}]`), format samples (`formats: [{path, dialect}]`), readout rules (`readouts: [...]`, each rule's own `test` entries run through the readout loader, so a rule with no fixture, a failing one or a stage that drops lines is withheld and named, counted per rule). A section with no fixtures or a failing one is withheld and named. Exit 0 when every present section passes, 1 when one is withheld, 2 on an unreadable or invalid pack. Nothing is executed.
