// valid.cue — positive fixtures for the heartbeat schema.
package heartbeat

beat1: #Beat & {
	channel: "edit-hook"
	kind:    "seen"
	at:      "2026-07-16T08:00:00Z"
	meta: {service: "fathom"}
}

beat2: #Beat & {
	channel: "session-hook"
	at:      "2026-07-16T08:00:01Z"
}

exp1: #Expectation & {
	if_channel:     "edit-hook.seen"
	then_channel:   "edit-hook.emitted"
	within_seconds: 86400
}
