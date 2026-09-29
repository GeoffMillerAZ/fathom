package mcp

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// TestClaimIDsForCallerStripsForEveryoneButTheEditHook is SR-61's payload
// bar at the function that enforces it (surface reckoning F-27/U4): the
// claim id is served to origin edit-hook and to no one else, and the
// caller's slice is never edited in place (the backend's rows may be a
// shared memo).
func TestClaimIDsForCallerStripsForEveryoneButTheEditHook(t *testing.T) {
	rules := []semantic.Obligation{
		{Rule: "a", Src: "d.md#L1-L1", ClaimID: "id-a"},
		{Rule: "raw fallback row", Src: "d.md#L2-L2"}, // no claim behind it
	}
	for _, origin := range []string{"", "session-hook", "prompt-hook", "doctor", OriginConsole, "verify-cli"} {
		got := claimIDsForCaller(WithOrigin(context.Background(), origin), rules)
		for i, r := range got {
			if r.ClaimID != "" {
				t.Errorf("origin %q: rules[%d] carries claim id %q", origin, i, r.ClaimID)
			}
		}
		if got[0].Rule != "a" || got[1].Rule != "raw fallback row" {
			t.Errorf("origin %q: rules reordered or rewritten: %+v", origin, got)
		}
	}
	if rules[0].ClaimID != "id-a" {
		t.Error("claimIDsForCaller edited the caller's slice in place")
	}
	hook := claimIDsForCaller(WithOrigin(context.Background(), OriginEditHook), rules)
	if hook[0].ClaimID != "id-a" {
		t.Errorf("the edit hook was not served the claim id: %+v", hook)
	}
}
