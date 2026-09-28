package console

import (
	"reflect"
	"strings"
	"testing"
)

func frictionFixture() FrictionInputs {
	ev := []ActivityEvent{
		// s1: five lookups in a row ending at pay.Settle, then an edit.
		{Session: "s1", At: "2026-09-02T10:00:01Z", Kind: ActSearch, File: "internal/pay/ledger.go", Symbol: "pay.Post"},
		{Session: "s1", At: "2026-09-02T10:00:02Z", Kind: ActLookup, File: "internal/pay/ledger.go", Symbol: "pay.Post"},
		{Session: "s1", At: "2026-09-02T10:00:03Z", Kind: ActSearch},
		{Session: "s1", At: "2026-09-02T10:00:04Z", Kind: ActLookup, File: "internal/pay/settle.go", Symbol: "pay.Settle"},
		{Session: "s1", At: "2026-09-02T10:00:05Z", Kind: ActSearch, File: "internal/pay/settle.go", Symbol: "pay.Settle"},
		{Session: "s1", At: "2026-09-02T10:00:06Z", Kind: ActEdit, File: "internal/pay/settle.go"},
		{Session: "s1", At: "2026-09-02T10:00:07Z", Kind: ActRevert, File: "internal/pay/settle.go", Symbol: "pay.Settle"},
		// s2: four searches then an answer — not a loop.
		{Session: "s2", At: "2026-09-03T10:00:01Z", Kind: ActSearch, File: "internal/store/s.go", Symbol: "store.Open"},
		{Session: "s2", At: "2026-09-03T10:00:02Z", Kind: ActSearch, File: "internal/store/s.go", Symbol: "store.Open"},
		{Session: "s2", At: "2026-09-03T10:00:03Z", Kind: ActSearch},
		{Session: "s2", At: "2026-09-03T10:00:04Z", Kind: ActSearch},
		{Session: "s2", At: "2026-09-03T10:00:05Z", Kind: ActAnswer},
	}
	return FrictionInputs{
		Events: ev,
		Rules: []RuleEvent{
			{Rule: "R-money", Event: "blocked", Session: "s1", File: "internal/pay/settle.go", At: "2026-09-02T10:00:08Z"},
			{Rule: "R-money", Event: "blocked", Session: "s1", File: "internal/pay/settle.go", At: "2026-09-02T10:00:09Z"},
			{Rule: "R-log", Event: "served", Session: "s2", File: "internal/store/s.go", At: "2026-09-03T09:59:00Z"},
			{Rule: "R-log", Event: "violated", Session: "s2", File: "internal/store/s.go", At: "2026-09-03T10:00:06Z"},
			{Rule: "R-x", Event: "complied", Session: "s2", File: "internal/store/s.go"},
		},
		// A pointer to store.Open existed before s2 searched for it twice;
		// the pointer to pay.Post came after s1's lookups.
		Pointers: []DiscoveryPointer{{Symbol: "store.Open", At: "2026-09-01T00:00:00Z"}, {Symbol: "pay.Post", At: "2026-09-05T00:00:00Z"}},
		Unproven: []UnprovenAnchor{{Row: "svc|p.md|A-3", File: "internal/pay/settle.go", Symbol: "pay.Settle"}},
		Zones:    NewZones(nil, nil),
	}
}

func hotspot(hs []FrictionHotspot, key string) FrictionHotspot {
	for _, h := range hs {
		if h.Key == key {
			return h
		}
	}
	return FrictionHotspot{}
}

// WI-6 (§3 U3): friction per file, symbol and zone from the activity
// record — rediscovery, blocked edits, claimed-unproven, reverts, search
// loops.
func TestFrictionSignals(t *testing.T) {
	m := FrictionSignals(frictionFixture())
	settle := hotspot(m.Files, "internal/pay/settle.go")
	want := FrictionCounts{Blocked: 2, ClaimedUnproven: 1, Reverts: 1, SearchLoops: 1}
	if settle.Counts != want || settle.Total != 5 {
		t.Fatalf("settle.go counts = %+v (total %d), want %+v", settle.Counts, settle.Total, want)
	}
	if len(settle.Blocks) != 1 || settle.Blocks[0] != (RuleCount{Rule: "R-money", N: 2}) {
		t.Fatalf("blocks by rule: %+v", settle.Blocks)
	}
	open := hotspot(m.Symbols, "store.Open")
	if open.Counts.Rediscovery != 2 || open.Counts.SearchLoops != 0 {
		t.Fatalf("store.Open was searched twice after its pointer existed; four searches then an answer is no loop: %+v", open.Counts)
	}
	if post := hotspot(m.Symbols, "pay.Post"); post.Counts.Rediscovery != 0 {
		t.Fatalf("a pointer written after the search is no rediscovery: %+v", post.Counts)
	}
	if s := hotspot(m.Symbols, "pay.Settle"); s.Counts.SearchLoops != 1 || !reflect.DeepEqual(s.LoopEnds, []string{"pay.Settle"}) {
		t.Fatalf("the loop is attributed to the symbol it ended at: %+v", s)
	}
	store := hotspot(m.Files, "internal/store/s.go")
	if store.Counts.Blocked != 1 || len(store.ServedViolations) != 1 || store.ServedViolations[0].Rule != "R-log" {
		t.Fatalf("an edit-hook violation of a served rule counts as blocked and is named: %+v", store)
	}
	pay := hotspot(m.Zones, "internal/pay")
	if pay.Total < settle.Total {
		t.Fatalf("the zone rolls up its files: %+v", pay)
	}
	if m.Files[0].Total < m.Files[len(m.Files)-1].Total {
		t.Fatal("hotspots sort by total, highest first")
	}
	for _, h := range m.Files {
		for _, e := range h.Evidence {
			if strings.Contains(e, "s1") || strings.Contains(e, "s2") {
				t.Fatalf("evidence names signals and times, never a session id: %q", e)
			}
		}
	}
	if m.Events != 12 || m.RuleEvents != 5 {
		t.Fatalf("the denominators are named: %d %d", m.Events, m.RuleEvents)
	}
}

// WI-7 (§3 U3): remedies from the pattern table, proposals with their
// evidence, never applied.
func TestFrictionRemediesAreProposals(t *testing.T) {
	in := frictionFixture()
	m := FrictionSignals(in)
	before := FrictionSignals(in)
	rems := frictionRemedies(m, RemedyInputs{
		RuleText:  map[string]string{"R-money": "money moves through pay.Post only"},
		RefCounts: map[string]int{"pay.Settle": 0},
		StaleDocs: []StaleDocSpan{{Doc: "docs/pay.md", Span: "L10-L20", Near: "internal/pay"}},
	})
	if !reflect.DeepEqual(m, before) {
		t.Fatal("computing remedies changed the map")
	}
	patterns := map[string]Remedy{}
	for _, r := range rems {
		if r.Status != RemedyProposed {
			t.Fatalf("every remedy is a proposal, never applied: %+v", r)
		}
		if len(r.Evidence) == 0 {
			t.Fatalf("every remedy carries its evidence: %+v", r)
		}
		patterns[r.Pattern+"@"+r.Target] = r
	}
	blk, ok := patterns[PatternRepeatedBlocks+"@internal/pay/settle.go"]
	if !ok || !strings.Contains(strings.Join(blk.Evidence, " "), "money moves through pay.Post only") {
		t.Fatalf("repeated blocks show the rule's text: %+v", rems)
	}
	if _, ok := patterns[PatternRediscovery+"@store.Open"]; !ok {
		t.Fatalf("rediscovery proposes an anchor or pointer: %+v", rems)
	}
	if r, ok := patterns[PatternServedIgnored+"@internal/store/s.go"]; !ok || !strings.Contains(r.Proposal, "hook") {
		t.Fatalf("a violated served rule proposes a hook: %+v", rems)
	}
	if _, ok := patterns[PatternStaleDoc+"@internal/pay"]; !ok {
		t.Fatalf("a stale doc near the zone proposes a refresh: %+v", rems)
	}
	if _, ok := patterns[PatternLoopNoRefs+"@pay.Settle"]; !ok {
		t.Fatalf("a loop ending at an unreferenced symbol proposes an anchor: %+v", rems)
	}
	if len(frictionRemedies(FrictionMap{}, RemedyInputs{})) != 0 {
		t.Fatal("no friction, no remedy")
	}
}
