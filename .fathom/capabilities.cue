// .fathom/capabilities.cue — which fathom features are on in this repository.
//
// Uncomment a line to override fathom's default. This file GOVERNS: a
// setting here beats anything in ~/.fathom/capabilities.cue (law 13).
//
// Run `fathom capabilities` to see what is on, and `fathom capabilities
// explain <id>` for what one of them costs.
//
// capabilities: {
//     agent-surface: on: false // The curated set of tools an AI coding agent sees
//     guards:        on: false // Extra, repository-specific guidance attached to particular paths — shown before an edit or a command touching them, for the files where getting it wrong is expensive
//     index:         on: false // Fathom holds a searchable index of this repository's symbols, so an agent can ask where something is defined instead of reading files to find it
//     layers:        on: false // Semantic layer packs — reusable sets of claims about a technology or house style, compiled over this repository's corpus
//     modules:       on: false // Published expert modules — curated domain knowledge from outside this repository — answerable from inside it
//     narrate:       on: true  // Captures short stories of what happened in a session alongside the counts
//     obligations:   on: false // Before an agent edits a file, fathom puts the rules governing that file in front of it — so a rule is applied while the code is written rather than found in review
//     session-ledger: on: false // Records per-session counts — sessions, discovery actions, gate runs — so fathom can report what it did rather than assert it
//     verify-gate:   on: false // Before a push, fathom re-checks every "done" box in this repository's plans against the code, and fails when the code does not back the claim
// }
