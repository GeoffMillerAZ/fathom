# ui-appkit — kill gate

**Status: authored 2026-08-07, not yet compiled anywhere.** Enters its
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
