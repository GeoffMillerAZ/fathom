package authorship_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/authorship"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// fixture commits — a canned "git history" (E2-2: "fixture git history")
// exercising: a plain trailer, a case-variant trailer key, two co-authors on
// one commit, and a commit with no trailer at all (must yield zero claims).
var fixtureCommits = []store.Commit{
	{
		ServiceID: "svc",
		Hash:      "aaa1111111111111111111111111111111111a",
		Author:    "Alice",
		Date:      300,
		Message:   "Fix auth flow\n\nLonger explanation of the change.\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>",
	},
	{
		ServiceID: "svc",
		Hash:      "bbb2222222222222222222222222222222222b",
		Author:    "Bob",
		Date:      200,
		Message:   "Refactor util package\n\nNo behavior change.",
	},
	{
		ServiceID: "svc",
		Hash:      "ccc3333333333333333333333333333333333c",
		Author:    "Carol",
		Date:      100,
		// Lowercase trailer key (git trailers are case-insensitive) plus a
		// second co-author on the same commit.
		Message: "Add widget\n\nco-authored-by: Dana Smith <dana@example.com>\nCo-authored-by: Erin Lee <erin@example.com>",
	},
}

func TestCompileClaims_TrailerFound(t *testing.T) {
	watermark := "deadbeef"
	at := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	claims, links := authorship.CompileClaims("svc", fixtureCommits, watermark, at)

	if len(links) != 0 {
		t.Errorf("expected no claim links (evidence lives in Rule, not a citation), got %d", len(links))
	}
	// aaa: 1 trailer, bbb: 0, ccc: 2 trailers = 3 claims total.
	if len(claims) != 3 {
		t.Fatalf("expected 3 claims, got %d: %+v", len(claims), claims)
	}
	for _, c := range claims {
		if c.Layer != authorship.LayerAuthorship {
			t.Errorf("Layer = %q, want %q", c.Layer, authorship.LayerAuthorship)
		}
		if c.Kind != authorship.KindAuthorship {
			t.Errorf("Kind = %q, want %q", c.Kind, authorship.KindAuthorship)
		}
		if c.Tier != core.TierVerified {
			t.Errorf("Tier = %q, want %q (Model-empty lexical oracle, ADR-018)", c.Tier, core.TierVerified)
		}
		if c.Model != "" {
			t.Errorf("Model = %q, want empty (deterministic pass, ADR-016 §Provenance)", c.Model)
		}
		if c.Status != core.ClaimFresh {
			t.Errorf("Status = %q, want fresh", c.Status)
		}
		if c.CompiledAtWatermark != watermark {
			t.Errorf("CompiledAtWatermark = %q, want %q", c.CompiledAtWatermark, watermark)
		}
		if !c.CompiledAt.Equal(at) {
			t.Errorf("CompiledAt = %v, want %v", c.CompiledAt, at)
		}
		if c.ClaimID == "" {
			t.Error("ClaimID must not be empty")
		}
	}
}

func TestCompileClaims_SubjectIsCommitSHA(t *testing.T) {
	claims, _ := authorship.CompileClaims("svc", fixtureCommits, "wm", time.Now())
	for _, c := range claims {
		switch c.Subject {
		case "aaa1111111111111111111111111111111111a", "ccc3333333333333333333333333333333333c":
			// expected
		default:
			t.Errorf("unexpected claim subject %q (want a commit sha)", c.Subject)
		}
	}
}

func TestCompileClaims_RuleIsTrailerLine(t *testing.T) {
	claims, _ := authorship.CompileClaims("svc", fixtureCommits[:1], "wm", time.Now())
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	want := "Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
	if claims[0].Rule != want {
		t.Errorf("Rule = %q, want %q (evidence = the trailer line)", claims[0].Rule, want)
	}
}

func TestCompileClaims_NoTrailer_NoClaim(t *testing.T) {
	claims, _ := authorship.CompileClaims("svc", fixtureCommits[1:2], "wm", time.Now())
	if len(claims) != 0 {
		t.Fatalf("expected 0 claims for a commit with no trailer, got %d", len(claims))
	}
}

func TestCompileClaims_MultipleCoAuthorsOneCommit(t *testing.T) {
	claims, _ := authorship.CompileClaims("svc", fixtureCommits[2:3], "wm", time.Now())
	if len(claims) != 2 {
		t.Fatalf("expected 2 claims (two co-authors on one commit), got %d", len(claims))
	}
	ids := map[string]bool{}
	for _, c := range claims {
		if ids[c.ClaimID] {
			t.Errorf("duplicate ClaimID %q — two trailers on one commit collided", c.ClaimID)
		}
		ids[c.ClaimID] = true
	}
}

func TestCompileClaims_Deterministic(t *testing.T) {
	at := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	claims1, links1 := authorship.CompileClaims("svc", fixtureCommits, "wm", at)
	claims2, links2 := authorship.CompileClaims("svc", fixtureCommits, "wm", at)
	if len(claims1) != len(claims2) {
		t.Fatalf("non-deterministic claim count: %d vs %d", len(claims1), len(claims2))
	}
	for i := range claims1 {
		if !reflect.DeepEqual(claims1[i], claims2[i]) {
			t.Errorf("claim %d differs across runs:\n%+v\n%+v", i, claims1[i], claims2[i])
		}
	}
	if len(links1) != len(links2) {
		t.Fatalf("non-deterministic link count: %d vs %d", len(links1), len(links2))
	}
	// Order must not depend on input commit order (commits arrive newest
	// first from ListCommits, but claim identity is content-addressed).
	shuffled := []store.Commit{fixtureCommits[2], fixtureCommits[0], fixtureCommits[1]}
	claims3, _ := authorship.CompileClaims("svc", shuffled, "wm", at)
	if len(claims3) != len(claims1) {
		t.Fatalf("claim count depends on input order: %d vs %d", len(claims3), len(claims1))
	}
	for i := range claims1 {
		if claims1[i].ClaimID != claims3[i].ClaimID {
			t.Errorf("claim order/identity depends on input commit order at index %d: %q vs %q", i, claims1[i].ClaimID, claims3[i].ClaimID)
		}
	}
}

// TestCompileClaims_DuplicateTrailerLinesDeduped is the F4 regression:
// cherry-pick/squash tooling routinely leaves two byte-identical
// Co-Authored-By lines in one commit. Both share a content-addressed
// ClaimID, so the store would upsert them into a single row — CompileClaims
// must mint ONE claim, keeping "claims emitted" equal to what persists.
func TestCompileClaims_DuplicateTrailerLinesDeduped(t *testing.T) {
	dup := []store.Commit{{
		ServiceID: "svc",
		Hash:      "ddd4444444444444444444444444444444444444",
		Author:    "Alice",
		Date:      400,
		Message:   "Squash of two commits\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>",
	}}
	claims, _ := authorship.CompileClaims("svc", dup, "wm", time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC))
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim for duplicate identical trailer lines, got %d: %+v", len(claims), claims)
	}
	// A DIFFERENT trailer line in the same commit must still mint its own claim.
	dup[0].Message += "\nCo-Authored-By: Erin Lee <erin@example.com>"
	claims2, _ := authorship.CompileClaims("svc", dup, "wm", time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC))
	if len(claims2) != 2 {
		t.Fatalf("expected 2 claims (dedupe must not collapse distinct co-authors), got %d", len(claims2))
	}
}

func TestCompileClaims_EmptyInput(t *testing.T) {
	claims, links := authorship.CompileClaims("svc", nil, "wm", time.Now())
	if len(claims) != 0 || len(links) != 0 {
		t.Errorf("expected no claims/links for empty commit list, got %d/%d", len(claims), len(links))
	}
}

func TestParseTrailers(t *testing.T) {
	body := "Subject line\n\nBody prose.\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>\nSigned-off-by: Alice <alice@example.com>"
	trailers := authorship.ParseTrailers(body)
	if len(trailers) != 1 {
		t.Fatalf("expected 1 Co-Authored-By trailer (Signed-off-by is a different trailer key), got %d: %+v", len(trailers), trailers)
	}
	if trailers[0].Name != "Claude Fable 5" {
		t.Errorf("Name = %q, want %q", trailers[0].Name, "Claude Fable 5")
	}
	if trailers[0].Email != "noreply@anthropic.com" {
		t.Errorf("Email = %q, want %q", trailers[0].Email, "noreply@anthropic.com")
	}
	if trailers[0].Line != "Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>" {
		t.Errorf("Line = %q, want the verbatim trailer line", trailers[0].Line)
	}
}

func TestParseTrailers_NoTrailer(t *testing.T) {
	if got := authorship.ParseTrailers("Subject only, no body"); len(got) != 0 {
		t.Errorf("expected 0 trailers, got %d", len(got))
	}
}

func TestParseTrailers_Empty(t *testing.T) {
	if got := authorship.ParseTrailers(""); len(got) != 0 {
		t.Errorf("expected 0 trailers for empty body, got %d", len(got))
	}
}
