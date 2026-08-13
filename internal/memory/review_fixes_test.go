package memory

// review_fixes_test.go — W1 and W2 from the adversarial review, pinned.

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestAmbiguousAnchorIsRefusedAtWrite pins W1's write half: a bare name
// colliding across packages must refuse to bind, naming the qualified
// candidates — not record a baseline for whichever candidate an unordered
// store fold happened to surface first.
func TestAmbiguousAnchorIsRefusedAtWrite(t *testing.T) {
	idx := newFakeIndex("g1")
	idx.setAmbiguous("symbol:New", "svc::pkga::New", "svc::pkgb::New")

	_, err := WriteMemory(context.Background(), NewMemStore(), idx, fixedClock(time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)), Draft{
		Service:  "svc",
		Claim:    "New validates its config before returning.",
		Anchors:  []Anchor{{Kind: AnchorSymbol, Ref: "New"}},
		Evidence: []Evidence{{Kind: EvidenceIndex, Ref: "internal/pkga/new.go"}},
		Author:   Actor{Kind: ActorHuman, Name: "reviewer"},
	})
	if err == nil {
		t.Fatal("an ambiguous bare-name anchor must be refused at write, not bound to an arbitrary winner")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("the refusal must say WHY: %v", err)
	}
}

// TestAmbiguityIsSuspectNotFresh pins W1's read half plus W2's rule: drift
// that cannot be judged — an anchor gone ambiguous, or one with no
// recorded baseline — must never report fresh. Fresh's documented contract
// is "every anchor's hash matches what it was at the last confirmation",
// and neither case can make that claim.
func TestAmbiguityIsSuspectNotFresh(t *testing.T) {
	idx := newFakeIndex("g1")
	idx.set("symbol:parseConfig", "h1")
	at := mustTime(t, "2026-08-12T12:00:00Z")

	m := Memory{
		ID: "m1", Service: "svc", Claim: "parseConfig validates the schema.",
		Anchors:      []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
		AnchorHashes: map[string]string{"symbol:parseConfig": "h1"},
		Tier:         "verified", Confidence: 5,
	}

	t.Run("baseline present and matching is fresh", func(t *testing.T) {
		ev, err := EvaluateState(context.Background(), idx, nil, fixedClock(at), m)
		if err != nil {
			t.Fatal(err)
		}
		if ev.State != StateFresh {
			t.Fatalf("state = %s, want fresh (the control case)", ev.State)
		}
	})

	t.Run("a later package collision makes it SUSPECT with a labeled reason", func(t *testing.T) {
		idx.setAmbiguous("symbol:parseConfig", "svc::alpha::parseConfig", "svc::beta::parseConfig")
		ev, err := EvaluateState(context.Background(), idx, nil, fixedClock(at), m)
		if err != nil {
			t.Fatal(err)
		}
		if ev.State != StateSuspect {
			t.Fatalf("state = %s, want suspect — an anchor that can no longer be watched cannot be fresh", ev.State)
		}
		if !ev.Surfaces {
			t.Error("suspect surfaces, labeled — suppressing it silently loses more than it saves")
		}
		if ev.Tier == "verified" {
			t.Error("law 14: the tier must drop while the verification cannot re-run")
		}
		var found bool
		for _, d := range ev.Drift {
			if d.Status == DriftUnknown && strings.Contains(d.Detail, "fully-qualified") {
				found = true
			}
		}
		if !found {
			t.Errorf("the drift detail must name the repair (re-anchor with a qualified key): %+v", ev.Drift)
		}
	})
}

// TestNoBaselineIsNeverFresh pins W2 exactly as the reviewer ran it: a
// memory with empty AnchorHashes at verified/5 reported state=fresh,
// surfaces=true, tier=verified, confidence=5 — a measurement that was
// never made, rendered as fact on the four fields injection and search
// sort by.
func TestNoBaselineIsNeverFresh(t *testing.T) {
	idx := newFakeIndex("g1")
	idx.set("symbol:parseConfig", "h1")

	m := Memory{
		ID: "m2", Service: "svc", Claim: "parseConfig validates the schema.",
		Anchors:      []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
		AnchorHashes: map[string]string{}, // the reviewer's exact shape
		Tier:         "verified", Confidence: 5,
	}
	ev, err := EvaluateState(context.Background(), idx, nil, fixedClock(mustTime(t, "2026-08-12T12:00:00Z")), m)
	if err != nil {
		t.Fatal(err)
	}
	if ev.State == StateFresh {
		t.Fatal("fresh claims every anchor MATCHES its last confirmation; with no baseline nothing was compared")
	}
	if ev.State != StateSuspect {
		t.Fatalf("state = %s, want suspect — surfaced, labeled, penalised (the noisy direction)", ev.State)
	}
	if ev.Tier == "verified" || ev.Confidence >= 5 {
		t.Errorf("tier/confidence = %s/%d — an unverifiable memory must not keep an unpenalised tier (law 3)",
			ev.Tier, ev.Confidence)
	}
}
