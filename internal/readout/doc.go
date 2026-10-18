// Package readout is the readouts engine (ADR-071, goal G39; design
// docs/design/readouts.md, plan docs/plans/readouts.md): a command's output
// shown to the agent as a compact, enriched or raw readout that ends in a
// receipt, a view of tool output and never a command runner.
//
// The API the hook (RO-16) and the CLI verbs (RO-14) build on:
//
//   - LoadRules(fathomDir) loads the rule set (RO-7): the embedded built-ins
//     (rules/*.toml, in the user format; RO-8 to RO-13 add them), then
//     fathomDir/readouts.toml, then fathomDir/readouts.d/*.toml in name
//     order. Production passes fathomhome.FathomHome(); a test passes its
//     own directory. Each file is TOML (`schema = 1`, `[[rule]]`) validated
//     by the embedded CUE schema (schema/readouts/schema.cue, unknown keys
//     refused, no field naming an executable, argv, shell text or URL,
//     joins from the closed built-in table). A rule is served only when it
//     compiles and keeps the contract: a lossy rule names a recovery and
//     carries a protected-line fixture for each stream its match reaches (a
//     trip in a diagnostic stream; an error or path:line line dropped with
//     no trip in a content stream), every fixture passes twice with the
//     same bytes, and every lossless stage round-trips through its inverse.
//     Anything else is in RuleSet.Refused, named by source, id and reason.
//     A later rule replaces an earlier one with the same id.
//
//   - Render(rules, Input) is the engine (RO-5), a pure function: no I/O,
//     no clock, no daemon. Input carries the command lexed by
//     shellcmd.ParseCommand, stdout, stderr, the interrupted flag, the
//     harness cap, the persisted or saved raw file, the configured mode,
//     and the built-in joins' lines, which the caller computes. Render
//     classifies the command (unreadable, a FATHOM_READOUT=raw prefix, an
//     rtk command, an interrupted run and machine-bound shapes stay raw),
//     matches the last matching rule, classes stdout as content or
//     diagnostic from a closed table in code (never a rule), runs the
//     stages, then the records, then the joins, then the contract: a
//     protected line that would drop serves the raw output with a receipt
//     naming the trip; a lean readout not smaller than the raw is not
//     served (never-worse). Output bytes are deterministic, and every
//     served readout ends with a receipt naming rule@version, mode, the
//     lines dropped by class and the recovery. Readout carries the view
//     and what the hook row records (rule, mode, stream, reason, bytes and
//     lines in and out, dropped by class, trips, records, joins missed).
//
//   - Lossiness(stages) is a rule's lossiness computed from the closed
//     stage set (RO-6): none (ansi, collapse, group, extract, join),
//     columns (fields), lines (keep, drop, cap) or whole (summarize,
//     outline), never declared by an author.
//
// Rule file fields beyond design §7's table: a fixture may name
// `input_file` (a file beside the rule) instead of `input`, `stream`
// ("content" or "diagnostic", only one the rule's match reaches) and
// `trip = true` (a protected line would drop, so raw is served); a stage
// may name `keep` (the captures a fields stage keeps, instead of a
// template).
package readout
