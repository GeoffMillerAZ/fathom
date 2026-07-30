package main

import "os"

// channels.go — the FATHOM_CHANNELS=off kill switch
// (docs/design/production-tokenomics.md §9.4 item 4 / §5's pre-registered
// A/B protocol OFF arm: "hooks silent, MCP surface unregistered").
//
// Every fathom hook (session-hook, edit-hook pre/post including the
// discovery observer, stop-hook) calls fathomChannelsDisabled() FIRST,
// before any other work — stdin read, HTTP call, or beat emission — and
// exits in total silence when it reports true. This is what makes the OFF
// arm a clean baseline: no injected context, no instrumentation traffic,
// nothing measurable as fathom activity for that session.
//
// Honesty (the O15 failure mode this must not repeat: a silent, undetectable
// disablement). `fathom doctor` checks this SAME env var independently
// (doctor_phaseo.go's checkChannelsEnv) and warns loudly whenever it finds
// hooks disabled by environment. doctor itself is never silenced by
// FATHOM_CHANNELS — it is a diagnostic, not a hook, and the whole point of
// the doctor check is to make the kill switch's own state visible.
func fathomChannelsDisabled() bool {
	return os.Getenv("FATHOM_CHANNELS") == "off"
}
