# ui-appkit — kill gate

**Status: v2 corrected 2026-09-13; no passing real-model evaluation recorded.** Enters its
3-week generalize-or-kill window at first fleet compile.

**Metric:** from hook telemetry — the fraction of edits to ui-appkit-scoped
files preceded by an obligations-channel call serving a ui-appkit claim,
AND an outcome signal: UI review findings citing a corpus/role/@source
violation the layer served (caught) vs. missed. The founding incident class
is measurable: sessions that hand-wrote components the corpus contained.

**Kill criterion:** fails on call-rate but not quality → fold the top three
rules into the guard pack's notes (guards need no compile) and kill the
layer. Fails on quality (claims drift from the kit's actual decisions, or
false positives train agents to ignore the channel) → kill; the substrate
cost was one pack directory.

**Owner:** app-kit. The pack versions with the kit; a kit decision change
that touches a compiled claim bumps the pack and recompiles consumers.

V2 excludes historical ADRs, private assets, assessments and plan claims from its
compile corpus. Structural guidance/parity canaries run in `go test ./cmd/appkit-mcp`;
they are not precision/recall evaluation. Keep `evaled: false` until labeled scope-trap
fixtures pass the actual Fathom real-model gate with recorded model/prompt/input identity.
Do not use `--unsafe-skip-eval-gate`. Estimate compilation before authorizing spend.
