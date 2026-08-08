package main

import "os"

// channels.go — the FATHOM_CHANNELS=off kill switch
// (docs/design/production-tokenomics.md §9.4 item 4 / §5's pre-registered
// A/B protocol OFF arm: "hooks silent, MCP surface unregistered").
//
// Every fathom hook (session-hook, edit-hook pre/post including the
// discovery observer, stop-hook) calls fathomChannelsDisabled() FIRST,
// before any other work — stdin read, HTTP call, or beat emission — and
// exits in total silence when it reports true.
//
// The OFF arm uses the NARROWER gate, assistiveChannelsDisabled(): no
// injected context and no assistive traffic, but the session ledger and
// the arm decrement still run, because an unmeasured control group is
// not a control group.
//
// Honesty (the O15 failure mode this must not repeat: a silent, undetectable
// disablement). `fathom doctor` checks this SAME env var independently
// (doctor_phaseo.go's checkChannelsEnv) and warns loudly whenever it finds
// hooks disabled by environment. doctor itself is never silenced by
// FATHOM_CHANNELS — it is a diagnostic, not a hook, and the whole point of
// the doctor check is to make the kill switch's own state visible.
// fathomChannelsDisabled is TOTAL silence: the env kill switch, and nothing
// else. It means "fathom does not run here at all" — no assistance, no
// measurement, no beat, no ledger row.
//
// The measurement arm is deliberately NOT part of it. Folding the two
// together (as this function did until 2026-08-08) was a self-defeating
// bug: every hook exits on this check before any other work, so an active
// OFF arm silenced the session ledger and the arm's own decrement along
// with the assistance. The result was an arm that burned its session
// budget collecting nothing — TokensOffSessions could never rise, and the
// counterfactual pilot could never leave "collecting". Measured cost of
// the bug: 14 hours of a real coding day recorded nowhere, with the
// budget still reading 10 of 10 because the decrement never ran either.
//
// arm.go's own header states the contract that was being violated: "the
// session ledger stamps every row with the arm it ran under, which is what
// makes paired comparison possible at all."
//
// Honesty (the O15 failure mode this must not repeat: a silent,
// undetectable disablement). `fathom doctor` checks this SAME env var
// independently (doctor_phasep.go's checkChannelsEnv) and warns loudly
// whenever it finds hooks disabled by environment. doctor itself is never
// silenced — it is a diagnostic, not a hook.
func fathomChannelsDisabled() bool {
	return os.Getenv("FATHOM_CHANNELS") == "off"
}

// assistiveChannelsDisabled gates everything that HELPS the agent —
// obligations, guards, the pre-edit channel, the verify gate at session
// end. It is true under the env kill switch and under an active OFF arm.
//
// The split is the whole point of vision 14 §2: an OFF arm is a control
// group, and a control group you do not measure is not a control group.
// The arm silences assistance so the session is genuinely unaided, and
// leaves measurement running so the session is genuinely comparable.
func assistiveChannelsDisabled() bool {
	return fathomChannelsDisabled() || armOffActive()
}
