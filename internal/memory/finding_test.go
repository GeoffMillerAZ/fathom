package memory

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func resolvedFinding() (core.SemanticClaim, []core.ClaimEvent) {
	claim := core.SemanticClaim{
		ClaimID:   "finding-1",
		ServiceID: "svc",
		Layer:     "findings",
		Kind:      FindingKind,
		Subject:   "arch_conformance#cfg.parseConfig",
		Rule:      "the nil check in parseConfig is load-bearing; removing it breaks the empty-config path.",
		Scope:     []string{"internal/cfg/cfg.go"},
		Status:    core.ClaimSuperseded,
		Tier:      core.TierVerified,
	}
	events := []core.ClaimEvent{{
		ClaimID:          "finding-1",
		Verdict:          "superseded",
		Author:           "review",
		ResolutionKind:   core.ResolutionCodeChange,
		ResolutionCommit: "commit:cccc333",
	}}
	return claim, events
}

// TestFindingBecomesMemory proves AM-7: a resolved finding becomes a memory
// with a symbol anchor and verified evidence, through the ordinary write path
// and with no new instrumentation.
func TestFindingBecomesMemory(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	idx.set("symbol:parseConfig", "hash-v1")
	idx.set("file:internal/cfg/cfg.go", "hash-file-v1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	claim, events := resolvedFinding()
	draft, err := MemoryFromFinding(claim, events, "parseConfig", Actor{})
	if err != nil {
		t.Fatalf("MemoryFromFinding: %v", err)
	}
	if draft.Service != "svc" || draft.Claim != claim.Rule {
		t.Errorf("draft = %+v, want the finding's service and rule", draft)
	}
	if draft.Commit != "commit:cccc333" {
		t.Errorf("draft.Commit = %q, want the resolution commit", draft.Commit)
	}

	var hasSymbolAnchor bool
	for _, a := range draft.Anchors {
		if a.Kind == AnchorSymbol && a.Ref == "parseConfig" {
			hasSymbolAnchor = true
		}
	}
	if !hasSymbolAnchor {
		t.Fatalf("anchors = %v, want a symbol anchor on parseConfig", draft.Anchors)
	}

	// EVERY piece of evidence must be machine-check: the finding itself and
	// the observed resolution. Asserting the exact set (rather than "at
	// least one is verified-grade") is what stops a single weaker entry
	// slipping in behind a stronger one and still deriving verified.
	wantEvidence := []string{
		"machine-check:finding:finding-1",
		"machine-check:resolution:code-change@commit:cccc333",
	}
	gotEvidence := make([]string, 0, len(draft.Evidence))
	for _, e := range draft.Evidence {
		gotEvidence = append(gotEvidence, e.Key())
	}
	sort.Strings(gotEvidence)
	if strings.Join(gotEvidence, " ") != strings.Join(wantEvidence, " ") {
		t.Fatalf("evidence = %v, want %v", gotEvidence, wantEvidence)
	}

	m, err := WriteMemory(ctx, ms, idx, fixedClock(at), draft)
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	if m.Tier != core.TierVerified {
		t.Errorf("Tier = %q, want %q — a machine-observed resolution is verified evidence", m.Tier, core.TierVerified)
	}
	if m.Capped {
		t.Errorf("Capped = true; the draft claims no tier at all, so nothing can be capped (%s)", m.CapReason)
	}
	if m.Author.Kind != ActorMachine {
		t.Errorf("Author.Kind = %q, want %q", m.Author.Kind, ActorMachine)
	}
}

// TestFindingRefusals: everything that is NOT a resolved, symbol-bearing
// finding is refused rather than turned into a plausible-looking memory.
func TestFindingRefusals(t *testing.T) {
	base, events := resolvedFinding()

	cases := []struct {
		name   string
		claim  func() core.SemanticClaim
		events []core.ClaimEvent
		symbol string
		want   RefusalReason
	}{
		{
			name:   "not a finding",
			claim:  func() core.SemanticClaim { c := base; c.Kind = "obligation"; return c },
			events: events, symbol: "parseConfig", want: RefusalBadEvidence,
		},
		{
			name:   "no symbol",
			claim:  func() core.SemanticClaim { return base },
			events: events, symbol: "  ", want: RefusalNoAnchor,
		},
		{
			name:   "no rule text",
			claim:  func() core.SemanticClaim { c := base; c.Rule = " "; return c },
			events: events, symbol: "parseConfig", want: RefusalNoClaim,
		},
		{
			name:   "still open",
			claim:  func() core.SemanticClaim { c := base; c.Status = core.ClaimFresh; return c },
			events: events, symbol: "parseConfig", want: RefusalNoEvidence,
		},
		{
			name:   "resolved only by its scope disappearing",
			claim:  func() core.SemanticClaim { return base },
			events: []core.ClaimEvent{{ClaimID: "finding-1", Verdict: "superseded", ResolutionKind: core.ResolutionScopeDisappearance}},
			symbol: "parseConfig", want: RefusalNoEvidence,
		},
		{
			name:   "no supersede event at all",
			claim:  func() core.SemanticClaim { return base },
			events: nil, symbol: "parseConfig", want: RefusalNoEvidence,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := MemoryFromFinding(tc.claim(), tc.events, tc.symbol, Actor{})
			re, ok := Refused(err)
			if !ok {
				t.Fatalf("err = %v, want a *RefusedError", err)
			}
			if re.Reason != tc.want {
				t.Errorf("Reason = %q, want %q (detail: %s)", re.Reason, tc.want, re.Detail)
			}
		})
	}
}

// TestFindingResolutionPicksTheNewestCodeChange: several supersede events,
// newest code-change wins, deterministically.
func TestFindingResolutionPicksTheNewestCodeChange(t *testing.T) {
	claim, _ := resolvedFinding()
	older := mustTime(t, "2026-07-01T00:00:00Z")
	newer := mustTime(t, "2026-08-01T00:00:00Z")
	events := []core.ClaimEvent{
		{ClaimID: "finding-1", Verdict: "superseded", ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "commit:old", At: older},
		{ClaimID: "finding-1", Verdict: "superseded", ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "commit:new", At: newer},
		{ClaimID: "other", Verdict: "superseded", ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "commit:wrong", At: newer},
	}
	draft, err := MemoryFromFinding(claim, events, "parseConfig", Actor{})
	if err != nil {
		t.Fatalf("MemoryFromFinding: %v", err)
	}
	if draft.Commit != "commit:new" {
		t.Errorf("Commit = %q, want commit:new", draft.Commit)
	}
	var refs []string
	for _, e := range draft.Evidence {
		refs = append(refs, e.Ref)
	}
	if !strings.Contains(strings.Join(refs, " "), "commit:new") {
		t.Errorf("evidence %v does not cite the resolving commit", refs)
	}
}
