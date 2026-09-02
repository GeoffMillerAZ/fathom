// invalid.cue — negative fixtures, documented commented-out so live
// `cue vet` stays green; the Go tests assert the behavior each protects.
package readmanifest

// 1. empty path:
// bad1: #Entry & {path: "", outcome: {kind: "read"}}

// 2. unreadable without the operating system's wording:
// bad2: #Outcome & {kind: "unreadable"}

// 3. skipped without a category:
// bad3: #Outcome & {kind: "skipped"}

// 4. unknown kind:
// bad4: #Outcome & {kind: "maybe"}

// 5. negative byte count:
// bad5: #Outcome & {kind: "read", bytes: -1}
