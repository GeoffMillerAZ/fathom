package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// locatingIndex is fakeIndex plus SymbolLocator: symbol ref → defining file.
type locatingIndex struct {
	*fakeIndex
	files map[string]string
}

func (l *locatingIndex) SymbolFile(_ context.Context, _ string, ref string) (string, bool, error) {
	f, ok := l.files[ref]
	return f, ok, nil
}

func TestMatchesQuery(t *testing.T) {
	m := Memory{
		Claim:   "parseConfig validates the schema before returning.",
		Anchors: []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}, {Kind: AnchorFile, Ref: "internal/cfg/parse.go"}},
	}
	cases := []struct {
		query string
		want  bool
	}{
		{"", true},
		{"SCHEMA", true},
		{"schema validates", true},
		{"parse.go", true},
		{"file:internal/cfg", true},
		{"schema retries", false},
		{"nope", false},
	}
	for _, c := range cases {
		if got := MatchesQuery(m, c.query); got != c.want {
			t.Errorf("MatchesQuery(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}

func TestSearchSurfacesOnlyLabeledLiveMemories(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	base := newFakeIndex("commit:aaaa111")
	idx := &locatingIndex{fakeIndex: base, files: map[string]string{"parseConfig": "internal/cfg/parse.go"}}
	at := mustTime(t, "2026-08-01T00:00:00Z")

	base.set("symbol:parseConfig", "h1")
	base.set("file:internal/cfg/parse.go", "h2")
	base.set("path:internal/cfg/**", "h3")
	base.set("file:cmd/x/main.go", "h4")

	write := func(claim string, anchors []Anchor, ev Evidence) Memory {
		t.Helper()
		m, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
			Service: "svc", Claim: claim, Anchors: anchors, Evidence: []Evidence{ev},
			Author: Actor{Kind: ActorHuman, Name: "alex"},
		})
		if err != nil {
			t.Fatalf("WriteMemory(%q): %v", claim, err)
		}
		return m
	}
	verified := write("parseConfig validates the schema before returning.",
		[]Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}}, Evidence{Kind: EvidenceMachineCheck, Ref: "TestParseConfig"})
	doc := write("The cfg package owns every config default.",
		[]Anchor{{Kind: AnchorPath, Ref: "internal/cfg/**"}}, Evidence{Kind: EvidenceDoc, Ref: "docs/cfg.md"})
	other := write("main wires the CLI.",
		[]Anchor{{Kind: AnchorFile, Ref: "cmd/x/main.go"}}, Evidence{Kind: EvidenceIndex, Ref: "svc::main"})
	retired := write("parse.go once retried on EOF.",
		[]Anchor{{Kind: AnchorFile, Ref: "internal/cfg/parse.go"}}, Evidence{Kind: EvidenceHuman, Ref: "alex"})
	if _, err := Resolve(ctx, ms, idx, fixedClock(at), retired.ID, ResolveNoLongerTrue, Actor{Kind: ActorHuman, Name: "alex"}, "", "", "removed"); err != nil {
		t.Fatalf("retire: %v", err)
	}

	// Everything live, in tier order: verified, pre-scan, doc-claimed.
	evs, err := Search(ctx, ms, idx, nil, fixedClock(at), SearchQuery{Service: "svc"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got, want := ids(evs), []string{verified.ID, other.ID, doc.ID}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for _, e := range evs {
		if e.Tier == "" || e.State == "" {
			t.Errorf("memory %s surfaced unlabelled: tier=%q state=%q", e.ID, e.Tier, e.State)
		}
		if e.ID == retired.ID {
			t.Errorf("retired memory %s surfaced", e.ID)
		}
	}

	// File scope: the symbol anchor is placed in parse.go by the locator,
	// the glob matches it, main.go's file anchor does not.
	evs, err = Search(ctx, ms, idx, nil, fixedClock(at), SearchQuery{Service: "svc", File: "internal/cfg/parse.go"})
	if err != nil {
		t.Fatalf("Search(file): %v", err)
	}
	if got, want := ids(evs), []string{verified.ID, doc.ID}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("file-scoped order = %v, want %v", got, want)
	}

	// Query narrows further, case-insensitively, over claim and anchors.
	evs, err = Search(ctx, ms, idx, nil, fixedClock(at), SearchQuery{Service: "svc", Query: "SCHEMA"})
	if err != nil {
		t.Fatalf("Search(query): %v", err)
	}
	if got := ids(evs); len(got) != 1 || got[0] != verified.ID {
		t.Fatalf("query = %v, want [%s]", got, verified.ID)
	}

	// Drift: a moved anchor surfaces suspect, labeled, tier dropped.
	base.set("symbol:parseConfig", "h1-moved")
	evs, err = Search(ctx, ms, idx, nil, fixedClock(at), SearchQuery{Service: "svc", Query: "schema"})
	if err != nil {
		t.Fatalf("Search(drifted): %v", err)
	}
	if len(evs) != 1 || evs[0].State != StateSuspect || evs[0].Tier != core.TierPreScan {
		t.Fatalf("drifted = %+v, want suspect at pre-scan", evs)
	}
	line := evs[0].ShortLine()
	if !strings.Contains(line, "[pre-scan · suspect ⚠]") || !strings.Contains(line, "symbol:parseConfig changed") {
		t.Errorf("ShortLine = %q, want label and the moved anchor", line)
	}

	// An orphaned anchor leaves the surfacing set (review queue, not payload).
	base.remove("symbol:parseConfig")
	evs, err = Search(ctx, ms, idx, nil, fixedClock(at), SearchQuery{Service: "svc"})
	if err != nil {
		t.Fatalf("Search(orphaned): %v", err)
	}
	for _, e := range evs {
		if e.ID == verified.ID {
			t.Errorf("orphaned memory %s surfaced", e.ID)
		}
	}
}

func TestSearchRefusesWithoutService(t *testing.T) {
	if _, err := Search(context.Background(), NewMemStore(), newFakeIndex("g"), nil, fixedClock(mustTime(t, "2026-08-01T00:00:00Z")), SearchQuery{}); err == nil {
		t.Fatal("Search without a service should refuse")
	}
}

func TestShortLineCarriesTierAndState(t *testing.T) {
	for _, st := range []State{StateFresh, StateSuspect, StateOrphaned, StateRetired} {
		ev := Evaluation{ID: "x", Claim: "a claim", State: st, Tier: core.TierSMEStated}
		line := ev.ShortLine()
		if !strings.Contains(line, string(st)) || !strings.Contains(line, string(core.TierSMEStated)) {
			t.Errorf("ShortLine %q for state %q omits tier or state", line, st)
		}
	}
}

func TestAnchorTouchesFile(t *testing.T) {
	ctx := context.Background()
	base := newFakeIndex("g")
	loc := &locatingIndex{fakeIndex: base, files: map[string]string{"parseConfig": "internal/cfg/parse.go"}}
	cases := []struct {
		idx  Index
		a    Anchor
		file string
		want bool
	}{
		{loc, Anchor{Kind: AnchorFile, Ref: "a.go"}, "a.go", true},
		{loc, Anchor{Kind: AnchorFile, Ref: "a.go"}, "b.go", false},
		{loc, Anchor{Kind: AnchorPath, Ref: "internal/**/*.go"}, "internal/cfg/parse.go", true},
		{loc, Anchor{Kind: AnchorPath, Ref: "cmd/**"}, "internal/cfg/parse.go", false},
		{loc, Anchor{Kind: AnchorSymbol, Ref: "parseConfig"}, "internal/cfg/parse.go", true},
		{loc, Anchor{Kind: AnchorSymbol, Ref: "parseConfig"}, "other.go", false},
		{base, Anchor{Kind: AnchorSymbol, Ref: "parseConfig"}, "internal/cfg/parse.go", false}, // no locator: cannot place
		{loc, Anchor{Kind: AnchorCommit, Ref: "abc"}, "internal/cfg/parse.go", false},
		{loc, Anchor{Kind: AnchorClaim, Ref: "c1"}, "internal/cfg/parse.go", false},
	}
	for _, c := range cases {
		got, err := AnchorTouchesFile(ctx, c.idx, "svc", c.a, c.file)
		if err != nil {
			t.Fatalf("AnchorTouchesFile(%s, %s): %v", c.a.Key(), c.file, err)
		}
		if got != c.want {
			t.Errorf("AnchorTouchesFile(%s, %s) = %v, want %v", c.a.Key(), c.file, got, c.want)
		}
	}
}
