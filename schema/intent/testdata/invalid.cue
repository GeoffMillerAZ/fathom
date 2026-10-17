// invalid.cue — negative fixtures for the intent schema, documented
// commented-out so live `cue vet` stays green (repo convention). Each case is
// asserted to actually fail by internal/intent/futures_test.go
// (TestIntentSchema_Negatives).
package intent

// 1. a Horizon outside next|later|someday (the 8 values ADR-074 repaired):
// bad1: #Future & {id: "F1", title: "x", confidence: "plausible", horizon: "soon"}

// 2. a Confidence on the retired scale:
// bad2: #Future & {id: "F2", title: "x", confidence: "high", horizon: "later"}

// 3. no Horizon at all (incomplete, so it fails a concrete vet; the Go test
//    validates concretely, as the gate does):
// bad3: #Future & {id: "F3", title: "x", confidence: "likely"}

// 4. a measured basis that cites nothing it measured:
// bad4: #Future & {id: "F4", title: "x", confidence: "likely", horizon: "next", basis: {kind: "measured", cite: "we looked"}}

// 5. a value without its why:
// bad5: #Future & {id: "F5", title: "x", confidence: "likely", horizon: "next", value: {level: "high", why: ""}}

// 6. a value level off the list:
// bad6: #Future & {id: "F6", title: "x", confidence: "likely", horizon: "next", value: {level: "huge", why: "y"}}

// 7. a key the parser never writes (#Future is closed):
// bad7: #Future & {id: "F7", title: "x", confidence: "likely", horizon: "next", effort: "low"}
