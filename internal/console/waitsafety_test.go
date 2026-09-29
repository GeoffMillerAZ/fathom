package console

import "time"

// waitSafety bounds a wait a test makes on progress: a channel a goroutine
// closes, a state a poll loop reads, a process that must exit. It is a hang
// guard sized for a machine starved for a minute (parallel gates under
// -race), never a latency the code under test is held to; the assertion is
// the progress itself, and a bound of seconds turns a slow scheduler into a
// red gate (EW-7).
const waitSafety = time.Minute
