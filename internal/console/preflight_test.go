package console

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// preflightHistory is a small proven history: two zones, a range of
// weights and retries, measured effort.
func preflightHistory() []ProvenChange {
	mk := func(i int, zone string, zones int, tokens float64, retries, weight int) ProvenChange {
		zs := []ZoneShare{{Zone: zone, Share: 1}}
		if zones == 2 {
			zs = []ZoneShare{{Zone: zone, Share: 0.5}, {Zone: "other/zone", Share: 0.5}}
		}
		return ProvenChange{
			Commit: fmt.Sprintf("c%03d", i), At: fmt.Sprintf("2026-09-%02dT10:00:00Z", i+1), Agent: "h|m|e",
			Known: true, Tokens: tokens, Turns: tokens / 1000, WallSeconds: tokens / 100, Weight: weight, Retries: retries, Zones: zs,
		}
	}
	return []ProvenChange{
		mk(0, "internal/console", 1, 40000, 0, 1),
		mk(1, "internal/console", 1, 42000, 0, 1),
		mk(2, "internal/console", 1, 90000, 2, 2),
		mk(3, "internal/console", 2, 60000, 1, 2),
		mk(4, "internal/console", 1, 44000, 0, 1),
		mk(5, "internal/console", 1, 46000, 0, 1),
		mk(6, "cmd/fathom", 1, 20000, 0, 1),
		mk(7, "cmd/fathom", 1, 21000, 0, 1),
		mk(8, "cmd/fathom", 1, 22000, 0, 1),
		mk(9, "internal/store", 1, 150000, 3, 3),
		mk(10, "internal/store", 1, 160000, 2, 3),
		mk(11, "internal/store", 1, 170000, 3, 3),
	}
}

func preflightRow() PreflightRow {
	return PreflightRow{Service: "svc", Plan: "docs/plans/p.md", ID: "X-1", Title: "a row", Symbol: "Thing", Package: "internal/console", Weight: 1}
}

func preflightInput() PreflightInput {
	changes := preflightHistory()
	return PreflightInput{
		Row:   preflightRow(),
		Blast: Present(BlastRadius{Callers: 12, Entrypoints: 1}, "the call graph"),
		Rules: Present([]GovernedRule{
			{Rule: "b rule", Level: RuleLevelWarn, File: "internal/console/x.go"},
			{Rule: "a rule", Level: RuleLevelError, File: "internal/console/x.go"},
			{Rule: "c rule", Level: RuleLevelInfo, File: "internal/console/a.go"},
		}, "the obligations layer"),
		Certs:    Present([]CertToReprove{{Subject: "symbol:Z", State: "live"}, {Subject: "symbol:A", State: "flaky"}}, "the certifications layer"),
		Zones:    []string{"internal/console", "cmd/fathom"},
		Interest: InterestRate(changes),
		Index:    rowFeatureIndex(changes),
	}
}

// WI-16: the difficulty profile is deterministic — the same facts in any
// order give the same profile, byte for byte — and every part names its
// truth state.
func TestDifficultyProfileDeterministic(t *testing.T) {
	in := preflightInput()
	p1 := DifficultyProfile(in)

	shuffled := preflightInput()
	shuffled.Rules = Present([]GovernedRule{in.Rules.V[2], in.Rules.V[0], in.Rules.V[1]}, "the obligations layer")
	shuffled.Certs = Present([]CertToReprove{in.Certs.V[1], in.Certs.V[0]}, "the certifications layer")
	shuffled.Zones = []string{"cmd/fathom", "internal/console", "cmd/fathom"}
	p2 := DifficultyProfile(shuffled)
	b1, err1 := json.Marshal(p1)
	b2, err2 := json.Marshal(p2)
	if err1 != nil || err2 != nil {
		t.Fatalf("a profile must marshal (every value carries a state): %v %v", err1, err2)
	}
	if string(b1) != string(b2) {
		t.Fatalf("the profile is deterministic over input order:\n%s\n%s", b1, b2)
	}
	if again, _ := json.Marshal(DifficultyProfile(in)); string(again) != string(b1) {
		t.Fatal("the same input twice gives the same profile")
	}

	// Blast radius passes through.
	if !p1.Blast.Known() || p1.Blast.V.Callers != 12 {
		t.Fatalf("blast radius: %+v", p1.Blast)
	}
	// Governed rules: strictest level first, then file, then rule; counts per level.
	if got := []string{p1.Rules.V[0].Rule, p1.Rules.V[1].Rule, p1.Rules.V[2].Rule}; !reflect.DeepEqual(got, []string{"a rule", "b rule", "c rule"}) {
		t.Fatalf("rules sort by level (error, warn, info), then file and rule: %v", got)
	}
	if p1.RuleLevels.Error != 1 || p1.RuleLevels.Warn != 1 || p1.RuleLevels.Info != 1 {
		t.Fatalf("level counts: %+v", p1.RuleLevels)
	}
	// Certificates to re-prove sort by subject.
	if p1.Certs.V[0].Subject != "symbol:A" {
		t.Fatalf("certificates sort by subject: %+v", p1.Certs.V)
	}
	// Zones are deduplicated and sorted; each carries the zone's interest
	// rate, or names why it has none.
	if len(p1.ZoneInterest) != 2 || p1.ZoneInterest[0].Zone != "cmd/fathom" || p1.ZoneInterest[1].Zone != "internal/console" {
		t.Fatalf("zones: %+v", p1.ZoneInterest)
	}
	if !p1.ZoneInterest[1].Rate.Known() || p1.MaxZoneRate.V != maxRate(p1.ZoneInterest) {
		t.Fatalf("the max zone rate is the largest known rate: %+v", p1.MaxZoneRate)
	}
	// K = 5 similar rows, effort measured, nearest first.
	if !p1.Similar.Known() || p1.Similar.V.K != PreflightK || len(p1.Similar.V.Rows) != PreflightK {
		t.Fatalf("k = %d similar rows: %+v", PreflightK, p1.Similar)
	}
	for i := 1; i < len(p1.Similar.V.Rows); i++ {
		if p1.Similar.V.Rows[i].Distance < p1.Similar.V.Rows[i-1].Distance {
			t.Fatal("similar rows sort nearest first")
		}
	}
	if p1.Similar.V.MedianTokens <= 0 {
		t.Fatalf("the similar rows' median effort: %+v", p1.Similar.V)
	}
	// Named features feed the Score.
	if len(p1.Features) != len(PreflightFeatureNames) || p1.Features[0].Name != PreflightFeatureNames[0] {
		t.Fatalf("features: %+v", p1.Features)
	}

	// With no facts at all every part is absent with a reason — never a 0.
	bare := DifficultyProfile(PreflightInput{Row: preflightRow()})
	if _, err := json.Marshal(bare); err != nil {
		t.Fatalf("a bare profile must marshal: %v", err)
	}
	for name, v := range map[string]State{"blast": bare.Blast.State, "rules": bare.Rules.State, "certs": bare.Certs.State, "similar": bare.Similar.State, "max zone rate": bare.MaxZoneRate.State} {
		if v != StateAbsent {
			t.Fatalf("%s with no facts is absent, got %s", name, v)
		}
	}
	if bare.Similar.Reason == "" || bare.Blast.Reason == "" {
		t.Fatal("an absent part names its reason")
	}
}

func maxRate(zs []ZoneRate) float64 {
	m := 0.0
	for _, z := range zs {
		if z.Rate.Known() && z.Rate.V > m {
			m = z.Rate.V
		}
	}
	return m
}

// The similar rows are the rows sharing the row's zones and shape, so a
// zone the row touches outweighs an identical shape elsewhere.
func TestSimilarRowsPreferSharedZones(t *testing.T) {
	changes := preflightHistory()
	ix := rowFeatureIndex(changes)
	got := ix.Similar(RowQuery{Zones: []string{"cmd/fathom"}, Weight: 1}, 3)
	if len(got) != 3 {
		t.Fatalf("k rows: %+v", got)
	}
	for _, r := range got {
		if r.SharedZones != 1 {
			t.Fatalf("the three cmd/fathom rows are nearest: %+v", got)
		}
	}
	if none := (&RowFeatureIndex{}).Similar(RowQuery{}, 5); len(none) != 0 {
		t.Fatalf("an empty index has no neighbours: %+v", none)
	}
}

// The blast radius of an anchor is its transitive callers, bounded.
func TestBlastRadiusFromCallers(t *testing.T) {
	callers := map[string][]string{"a": {"b", "c"}, "b": {"d"}, "c": {"d", "e"}, "d": {"main"}, "e": nil, "main": nil}
	of := func(k string) []string { return callers[k] }
	v := BlastRadiusOf("a", of, func(k string) bool { return k == "main" }, 100)
	if !v.Known() || v.V.Callers != 5 || v.V.Entrypoints != 1 || v.State == StatePartial {
		t.Fatalf("callers b c d e main, one entrypoint: %+v", v)
	}
	capped := BlastRadiusOf("a", of, nil, 3)
	if capped.State != StatePartial || capped.V.Callers != 3 || capped.Reason == "" {
		t.Fatalf("a capped walk is a lower bound, named: %+v", capped)
	}
	if lone := BlastRadiusOf("e", of, nil, 100); lone.State != StateZero && lone.State != StatePresent || lone.V.Callers != 0 {
		t.Fatalf("no callers is a measured zero: %+v", lone)
	}
}

// WI-17: the tier recommendation is a deterministic function of the
// profile through a published rule table, advisory, and cannot dispatch.
func TestRecommendTierAdvisoryOnly(t *testing.T) {
	calm := preflightInput()
	calm.Blast = Present(BlastRadius{Callers: 3}, "the call graph")
	calm.Rules = Present([]GovernedRule{}, "the obligations layer")
	calm.Certs = Present([]CertToReprove{}, "the certifications layer")
	calm.Zones = []string{"cmd/fathom"}

	cases := []struct {
		name  string
		edit  func(in *PreflightInput)
		tier  Tier
		fired []string
	}{
		{"calm row", func(in *PreflightInput) {}, TierLight, nil},
		{"wide blast radius", func(in *PreflightInput) { in.Blast = Present(BlastRadius{Callers: 120}, "the call graph") }, TierHeavy, []string{"T1"}},
		{"moderate blast radius", func(in *PreflightInput) { in.Blast = Present(BlastRadius{Callers: 30}, "the call graph") }, TierStandard, []string{"T2"}},
		{"error-level rule", func(in *PreflightInput) {
			in.Rules = Present([]GovernedRule{{Rule: "r", Level: RuleLevelError, File: "f.go"}}, "obligations")
		}, TierHeavy, []string{"T3"}},
		{"warn-level rule", func(in *PreflightInput) {
			in.Rules = Present([]GovernedRule{{Rule: "r", Level: RuleLevelWarn, File: "f.go"}}, "obligations")
		}, TierStandard, []string{"T4"}},
		{"certificates to re-prove", func(in *PreflightInput) {
			in.Certs = Present([]CertToReprove{{Subject: "symbol:A", State: "live"}}, "certifications")
		}, TierStandard, []string{"T6"}},
		{"already tried twice", func(in *PreflightInput) { in.Row.Retries = 2 }, TierHeavy, []string{"T9"}},
		{"heavy plan weight, and the heavy rows nearest it", func(in *PreflightInput) { in.Row.Weight = 6 }, TierStandard, []string{"T11", "T12", "T13"}},
		{"expensive zone", func(in *PreflightInput) { in.Zones = []string{"internal/store"} }, TierHeavy, []string{"T7"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := calm
			tc.edit(&in)
			p := DifficultyProfile(in)
			rec := recommendTier(p)
			if !rec.Advisory {
				t.Fatal("a recommendation is advisory, always")
			}
			if rec.Tier != tc.tier {
				t.Fatalf("tier %q, want %q: %+v", rec.Tier, tc.tier, rec)
			}
			var ids []string
			for _, f := range rec.Fired {
				ids = append(ids, f.ID)
				if f.Evidence == "" || f.When == "" {
					t.Fatalf("every fired rule cites its table row and its evidence: %+v", f)
				}
			}
			if !reflect.DeepEqual(ids, tc.fired) {
				t.Fatalf("fired %v, want %v: %+v", ids, tc.fired, rec)
			}
			if !rec.Complete {
				t.Fatalf("every rule was evaluable here: %+v", rec.Unevaluated)
			}
			if again := recommendTier(p); !reflect.DeepEqual(rec, again) {
				t.Fatal("deterministic")
			}
		})
	}

	// Unknown features never lower the tier and never claim "light": a
	// profile with no measured blast radius, rules, certificates or zone
	// interest has no recommendation, and a fired rule is a lower bound.
	bare := recommendTier(DifficultyProfile(PreflightInput{Row: preflightRow()}))
	if bare.Tier != "" || bare.Complete || len(bare.Unevaluated) == 0 || !bare.Advisory {
		t.Fatalf("nothing measured, nothing recommended: %+v", bare)
	}
	partial := recommendTier(DifficultyProfile(PreflightInput{Row: PreflightRow{ID: "X", Weight: 1, Retries: 2}}))
	if partial.Tier != TierHeavy || partial.Complete {
		t.Fatalf("a fired rule on partial facts is a floor, not a verdict: %+v", partial)
	}
	if !strings.Contains(partial.Note, "harness decides") {
		t.Fatalf("the note says who decides: %q", partial.Note)
	}

	// The recommendation is JSON with advisory set.
	b, err := json.Marshal(recommendTier(DifficultyProfile(calm)))
	if err != nil || !strings.Contains(string(b), `"advisory":true`) {
		t.Fatalf("wire shape: %s %v", b, err)
	}

	// The file that recommends can neither dispatch nor reach a network:
	// no exec or HTTP import, and no function that starts or routes work.
	_, self, _, _ := runtime.Caller(0)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(self), "preflight.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		switch strings.Trim(imp.Path.Value, `"`) {
		case "os/exec", "net/http", "net":
			t.Fatalf("preflight.go imports %s: preflight never dispatches", imp.Path.Value)
		}
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			for _, bad := range []string{"dispatch", "assign", "route", "launch", "spawn", "start"} {
				if strings.Contains(strings.ToLower(fn.Name.Name), bad) {
					t.Fatalf("preflight.go declares %s: preflight never starts, assigns or routes work (WI-X3)", fn.Name.Name)
				}
			}
		}
	}
}

// The rule table is published: every rule's id and condition appears in
// the design doc, and the table in code is what the doc shows.
func TestTierRuleTablePublished(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	doc, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "docs", "design", "work-intelligence.md"))
	if err != nil {
		t.Fatal(err)
	}
	rules := TierRules()
	if len(rules) < 8 {
		t.Fatalf("the published table: %+v", rules)
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if r.ID == "" || r.When == "" || r.Floor == "" || seen[r.ID] {
			t.Fatalf("a rule has an id, a condition, a floor and is listed once: %+v", r)
		}
		seen[r.ID] = true
		row := fmt.Sprintf("| %s | %s | %s |", r.ID, r.When, r.Floor)
		if !strings.Contains(string(doc), row) {
			t.Fatalf("docs/design/work-intelligence.md must publish the row %q", row)
		}
	}
}
