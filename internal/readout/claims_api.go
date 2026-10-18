package readout

// claims_api.go — the exported face of RO-37's closed claim table, which
// RO-39's stop claim check (cmd/fathom checkStopClaims) calls.

// Claim is one test or gate claim in an assistant message.
type Claim struct {
	// ID is the claim table entry that matched (all-tests-pass,
	// gate-passes, command-passes, tests-pass, green); a log names a claim
	// by it, never by the message's text.
	ID string
	// Invocation is the declared test command the claim names (TR-1's
	// invocation), or "" when any declared test command backs it.
	Invocation string
}

// ClaimsInMessage returns the claims in msg in message order, read from
// the closed table (claimsInMessage). declared is TR-1's matcher bound to
// the repository: whether a command runs a declared test command, and the
// declared invocation it ran; nil means none is declared, so a claim that
// names a command never matches. Pure and deterministic.
func ClaimsInMessage(msg string, declared func(command string) (invocation string, ok bool)) []Claim {
	found := claimsInMessage(msg, claimCommandMatcher(declared))
	out := make([]Claim, 0, len(found))
	for _, c := range found {
		out = append(out, Claim{ID: c.ID, Invocation: c.Invocation})
	}
	return out
}
