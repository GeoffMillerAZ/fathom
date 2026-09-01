# agentic-kit-standards — kill gate

**Status: authored 2026-08-07, not yet compiled anywhere.** Enters its
3-week generalize-or-kill window at first fleet compile.

**Metric:** from hook telemetry — the fraction of edits to scoped files
(go.mod, foundations/**, TRACKING.md, tests) preceded by an
obligations-channel call serving one of this pack's claims, AND an outcome
signal: gate failures / review findings citing a rule the layer served
(caught) vs. missed (a require-block addition, a schema/types drift, a
bare time.Now, a checked box the gate never proved).

**Kill criterion:** fails on call-rate but not quality → fold the top
rules into the guard pack's notes and kill the layer. Fails on quality
(claims drift from CLAUDE.md's actual text, or false positives erode the
channel) → kill; substrate cost was one pack directory.

**Owner:** agentic-kit. Pack versions with the kit; a hard-rule change
bumps the pack and recompiles consumers.
