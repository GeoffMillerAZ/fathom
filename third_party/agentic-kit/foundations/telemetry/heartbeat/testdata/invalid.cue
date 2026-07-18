// invalid.cue — negative fixtures, documented commented-out so live
// `cue vet` stays green; the Go test asserts each actually fails.
package heartbeat

// 1. empty channel:
// bad1: #Beat & {channel: "", at: "2026-07-16T08:00:00Z"}

// 2. missing at:
// bad2: #Beat & {channel: "edit-hook"}

// 3. non-positive window:
// bad3: #Expectation & {if_channel: "a", then_channel: "b", within_seconds: 0}
