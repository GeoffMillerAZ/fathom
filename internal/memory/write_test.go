package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestUnanchoredMemoryIsRefused proves AM-1's load-bearing half: a memory
// with no anchor is REFUSED with the reason — it is not stored at a low tier,
// not stored at all. The store is checked afterwards, because "refused" that
// still writes a row is the exact failure this design exists to prevent.
func TestUnanchoredMemoryIsRefused(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	_, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
		Service:  "svc",
		Claim:    "we prefer tabs over spaces",
		Anchors:  nil,
		Evidence: []Evidence{{Kind: EvidenceHuman, Ref: "alex"}},
		Author:   Actor{Kind: ActorHuman, Name: "alex"},
	})
	if err == nil {
		t.Fatal("WriteMemory accepted an unanchored memory; the refusal IS the feature")
	}
	re, ok := Refused(err)
	if !ok {
		t.Fatalf("error is not a *RefusedError: %T %v", err, err)
	}
	if re.Reason != RefusalNoAnchor {
		t.Errorf("Reason = %q, want %q", re.Reason, RefusalNoAnchor)
	}
	if !strings.Contains(re.Detail, "CLAUDE.md") {
		t.Errorf("refusal detail must name where the note belongs instead; got %q", re.Detail)
	}
	if !strings.Contains(re.Detail, "preference") {
		t.Errorf("refusal detail must say WHY (it is a preference); got %q", re.Detail)
	}

	// Not stored at a low tier. Not stored at all.
	all, lerr := ms.List(ctx, "")
	if lerr != nil {
		t.Fatalf("List: %v", lerr)
	}
	if len(all) != 0 {
		t.Fatalf("refused memory was stored anyway: %+v", all)
	}
}

// TestCeilingDerivedNotTrusted proves AM-2 with a LYING caller: it claims
// tier verified and confidence 5 while its only evidence is a model
// inference. The write path must derive the ceiling structurally
// (scoring.ApplyCeiling at the write path), land the memory at the ceiling,
// and record the cap and its reason.
func TestCeilingDerivedNotTrusted(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	idx.set("symbol:parseConfig", "hash-v1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	m, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
		Service:  "svc",
		Claim:    "parseConfig validates the schema before returning.",
		Anchors:  []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
		Evidence: []Evidence{{Kind: EvidenceModel, Ref: "some-model"}},
		// The lie.
		ClaimedTier:       core.TierVerified,
		ClaimedConfidence: 5,
		Author:            Actor{Kind: ActorModel, Name: "some-model"},
	})
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}

	if m.Tier != core.TierAssumed {
		t.Errorf("Tier = %q, want %q — the caller's claimed tier must not be honoured", m.Tier, core.TierAssumed)
	}
	if m.Confidence != 1 {
		t.Errorf("Confidence = %d, want 1 (the assumed ceiling; scoring.ApplyCeiling)", m.Confidence)
	}
	if !m.Capped {
		t.Error("Capped = false; a lying caller must be recorded as capped")
	}
	if m.CapReason == "" {
		t.Fatal("CapReason is empty; the cap must carry its reason")
	}
	for _, want := range []string{string(core.TierVerified), string(core.TierAssumed), "model"} {
		if !strings.Contains(m.CapReason, want) {
			t.Errorf("CapReason %q does not mention %q", m.CapReason, want)
		}
	}
	if m.ClaimedTier != core.TierVerified || m.ClaimedConfidence != 5 {
		t.Errorf("the claim must be retained for audit; got tier=%q confidence=%d", m.ClaimedTier, m.ClaimedConfidence)
	}

	// And the stored row carries the derived tier, not the claimed one.
	got, found, gerr := ms.Get(ctx, m.ID)
	if gerr != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, gerr)
	}
	if got.Tier != core.TierAssumed || got.Confidence != 1 {
		t.Errorf("stored tier/confidence = %q/%d, want %q/1", got.Tier, got.Confidence, core.TierAssumed)
	}
}

// TestCeilingHonoursStrongerEvidence is the other half of AM-2: the
// derivation is not a blanket downgrade. The same claim backed by a machine
// check lands at verified, uncapped — otherwise "derived, not trusted" would
// just mean "always distrusted", which is a different (and useless) rule.
func TestCeilingHonoursStrongerEvidence(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	idx.set("symbol:parseConfig", "hash-v1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	m, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
		Service: "svc",
		Claim:   "parseConfig validates the schema before returning.",
		Anchors: []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
		Evidence: []Evidence{
			{Kind: EvidenceMachineCheck, Ref: "TestParseConfigValidatesSchema"},
			{Kind: EvidenceIndex, Ref: "svc::cfg.parseConfig"},
		},
		ClaimedTier:       core.TierVerified,
		ClaimedConfidence: 5,
		Author:            Actor{Kind: ActorHuman, Name: "alex"},
	})
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	if m.Tier != core.TierVerified {
		t.Errorf("Tier = %q, want %q", m.Tier, core.TierVerified)
	}
	if m.Confidence != 5 {
		t.Errorf("Confidence = %d, want 5", m.Confidence)
	}
	if m.Capped {
		t.Errorf("Capped = true for a truthful caller; CapReason=%q", m.CapReason)
	}
}

// TestUnstatedConfidenceIsStillBounded: a caller that claims nothing gets a
// derived confidence, and that derivation is still bounded by the tier — an
// omitted claim must not be a way around the ceiling.
func TestUnstatedConfidenceIsStillBounded(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	idx.set("file:internal/cfg/cfg.go", "hash-v1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	m, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
		Service:  "svc",
		Claim:    "the loader rejects an unknown key rather than ignoring it.",
		Anchors:  []Anchor{{Kind: AnchorFile, Ref: "internal/cfg/cfg.go"}},
		Evidence: []Evidence{{Kind: EvidenceDoc, Ref: "ADR.md#config"}},
		Author:   Actor{Kind: ActorHuman, Name: "alex"},
	})
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	if m.Tier != core.TierDocClaimed {
		t.Errorf("Tier = %q, want %q", m.Tier, core.TierDocClaimed)
	}
	if m.Confidence > 2 {
		t.Errorf("Confidence = %d, want <= 2 (the doc-claimed ceiling)", m.Confidence)
	}
	if m.Capped {
		t.Errorf("Capped = true although the caller claimed nothing; CapReason=%q", m.CapReason)
	}
}

// TestWriteRefusals walks every other refusal so none of them can quietly
// become a silent low-tier write later.
func TestWriteRefusals(t *testing.T) {
	ctx := context.Background()
	at := mustTime(t, "2026-08-08T12:00:00Z")
	good := func() Draft {
		return Draft{
			Service:  "svc",
			Claim:    "a claim",
			Anchors:  []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
			Evidence: []Evidence{{Kind: EvidenceIndex, Ref: "svc::cfg.parseConfig"}},
			Author:   Actor{Kind: ActorHuman, Name: "alex"},
		}
	}

	cases := []struct {
		name   string
		mutate func(*Draft)
		want   RefusalReason
	}{
		{"no service", func(d *Draft) { d.Service = "  " }, RefusalNoService},
		{"no claim", func(d *Draft) { d.Claim = "\t" }, RefusalNoClaim},
		{"no evidence", func(d *Draft) { d.Evidence = nil }, RefusalNoEvidence},
		{"unknown anchor kind", func(d *Draft) { d.Anchors = []Anchor{{Kind: "vibe", Ref: "x"}} }, RefusalBadAnchor},
		{"empty anchor ref", func(d *Draft) { d.Anchors = []Anchor{{Kind: AnchorSymbol, Ref: " "}} }, RefusalBadAnchor},
		{"unknown evidence kind", func(d *Draft) { d.Evidence = []Evidence{{Kind: "hunch", Ref: "x"}} }, RefusalBadEvidence},
		{"empty evidence ref", func(d *Draft) { d.Evidence = []Evidence{{Kind: EvidenceIndex, Ref: ""}} }, RefusalBadEvidence},
		{"unknown author kind", func(d *Draft) { d.Author = Actor{Kind: "oracle"} }, RefusalBadActor},
		{"anchor absent from index", func(d *Draft) { d.Anchors = []Anchor{{Kind: AnchorSymbol, Ref: "neverExisted"}} }, RefusalAnchorUnresolved},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ms := NewMemStore()
			idx := newFakeIndex("gen-1")
			idx.set("symbol:parseConfig", "hash-v1")
			d := good()
			tc.mutate(&d)
			_, err := WriteMemory(ctx, ms, idx, fixedClock(at), d)
			re, ok := Refused(err)
			if !ok {
				t.Fatalf("err = %v, want a *RefusedError", err)
			}
			if re.Reason != tc.want {
				t.Errorf("Reason = %q, want %q (detail: %s)", re.Reason, tc.want, re.Detail)
			}
			all, _ := ms.List(ctx, "")
			if len(all) != 0 {
				t.Errorf("a refused write stored %d memories", len(all))
			}
		})
	}
}

// TestWriteIsIdempotentAndDeterministic: the same claim about the same
// anchors is the same memory, and the stored anchors/evidence are sorted
// regardless of the order the caller supplied them in.
func TestWriteIsIdempotentAndDeterministic(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	idx.set("symbol:parseConfig", "hash-v1")
	idx.set("file:internal/cfg/cfg.go", "hash-file-v1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	forward := Draft{
		Service: "svc",
		Claim:   "parseConfig validates the schema before returning.",
		Anchors: []Anchor{
			{Kind: AnchorSymbol, Ref: "parseConfig"},
			{Kind: AnchorFile, Ref: "internal/cfg/cfg.go"},
		},
		Evidence: []Evidence{
			{Kind: EvidenceIndex, Ref: "svc::cfg.parseConfig"},
			{Kind: EvidenceDoc, Ref: "ADR.md#config"},
		},
		Author: Actor{Kind: ActorHuman, Name: "alex"},
	}
	reversed := forward
	reversed.Anchors = []Anchor{forward.Anchors[1], forward.Anchors[0]}
	reversed.Evidence = []Evidence{forward.Evidence[1], forward.Evidence[0]}

	a, err := WriteMemory(ctx, ms, idx, fixedClock(at), forward)
	if err != nil {
		t.Fatalf("WriteMemory(forward): %v", err)
	}
	b, err := WriteMemory(ctx, ms, idx, fixedClock(at), reversed)
	if err != nil {
		t.Fatalf("WriteMemory(reversed): %v", err)
	}
	if a.ID != b.ID {
		t.Errorf("ids differ across anchor ordering: %s vs %s", a.ID, b.ID)
	}
	if a.Anchors[0].Key() != b.Anchors[0].Key() || a.Evidence[0].Key() != b.Evidence[0].Key() {
		t.Errorf("stored order is not canonical: %v / %v", b.Anchors, b.Evidence)
	}
	all, _ := ms.List(ctx, "")
	if len(all) != 1 {
		t.Errorf("re-adding the same memory produced %d rows, want 1", len(all))
	}
}

// TestPathAnchorIsWarnedNotRefused: §2.1 discourages path anchors in review
// without banning them, so the warning must be recorded and the write must
// still succeed.
func TestPathAnchorIsWarnedNotRefused(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	idx.set("path:internal/**/*.go", "hash-glob-v1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	m, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
		Service:  "svc",
		Claim:    "every internal package sorts its outputs.",
		Anchors:  []Anchor{{Kind: AnchorPath, Ref: "internal/**/*.go"}},
		Evidence: []Evidence{{Kind: EvidenceHuman, Ref: "alex"}},
		Author:   Actor{Kind: ActorHuman, Name: "alex"},
	})
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	if len(m.Warnings) != 1 || m.Warnings[0] != LooseAnchorWarning {
		t.Errorf("Warnings = %v, want exactly the loose-anchor warning", m.Warnings)
	}
}

// TestWriteRequiresAnInjectedClock: determinism is a contract, so the write
// path refuses rather than reaching for time.Now() on the caller's behalf.
func TestWriteRequiresAnInjectedClock(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	idx.set("symbol:parseConfig", "hash-v1")
	_, err := WriteMemory(ctx, ms, idx, nil, Draft{
		Service:  "svc",
		Claim:    "a claim",
		Anchors:  []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
		Evidence: []Evidence{{Kind: EvidenceIndex, Ref: "x"}},
	})
	if err == nil {
		t.Fatal("WriteMemory accepted a nil clock")
	}
	if !strings.Contains(err.Error(), "clock") {
		t.Errorf("error should name the missing clock; got %v", err)
	}
}
