// Package heartbeat defines the CUE-first contract for channel beats: the
// liveness substrate for forced delivery channels (hooks, gates, instruments).
// A beat records that a channel FIRED — activity signals ("edit-seen") beside
// effect signals ("injection-emitted") — so coverage has a denominator and a
// silent channel is detectable instead of invisible.
//
// CUE is authoritative for shape; Go types in heartbeat.go mirror these
// definitions; any drift is a bug.
//
// Honest scope claim (doc'd): beats are SIGNALS and liveness alarms are
// advisory data. CheckLiveness never exits a process, blocks a hook, or
// gates a push — consumers decide what an alarm means.
package heartbeat

// #Beat is one channel firing.
#Beat: {
	// channel names the instrumented channel ("edit-hook", "session-hook").
	channel: string & !=""

	// kind refines the event within the channel ("seen", "emitted", "empty",
	// "error"). Optional; empty means the channel's default event.
	kind?: string

	// at is the beat timestamp, RFC3339 UTC (injected clock, never wall time
	// at projection).
	at: string & !=""

	// meta carries small string labels (service, file class …). Values must
	// stay label-sized — beats are counters, not payloads.
	meta?: [string]: string
}

// #Expectation pairs an activity channel with the effect channel that must
// accompany it: "if IfChannel beat in the window, ThenChannel must have too."
#Expectation: {
	if_channel:   string & !=""
	then_channel: string & !=""
	// within_seconds is the window measured back from the reference time.
	within_seconds: int & >0
}
