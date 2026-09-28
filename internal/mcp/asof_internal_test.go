package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// asOfSeed is one seeded history: a rule and a finding that each stood from
// January to March and were replaced by a successor, and a doc link that was
// replaced the same day. Every verb is asked about the same three dates.
type asOfSeed struct {
	srv                *Server
	before, mid, after string // as_of arguments: before anything, mid-history, after the replacement
}

func newAsOfSeed(t *testing.T) asOfSeed {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/asof\n\ngo 1.22\n")
	write("pkg/a.go", "package pkg\n\nfunc A() { B() }\n")
	write("pkg/b.go", "package pkg\n\nfunc B() {}\n")

	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "asof.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if err := c.IngestServiceAt(ctx, "svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}

	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	claim := func(id, layer, kind, rule string, scope []string, status core.ClaimStatus, from, to time.Time) core.SemanticClaim {
		return core.SemanticClaim{
			ClaimID: id, ServiceID: "svc", Layer: layer, LayerVersion: "v1", Kind: kind,
			Subject: "docs/g.md#" + id, Rule: rule, Scope: scope,
			Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
			PromptVersion: "p", ReproKey: id, InputDigest: "d",
			CompiledAtWatermark: "manifest:aaa", CompiledAt: from, ValidFrom: from, ValidTo: to,
		}
	}
	oldRule := claim("rule-old", "obligations", "obligation", "Old rule.", []string{"**"}, core.ClaimSuperseded, jan, mar)
	newRule := claim("rule-new", "obligations", "obligation", "New rule.", []string{"**"}, core.ClaimFresh, mar, time.Time{})
	oldFinding := claim("finding-old", "findings", "finding", "Old finding.", []string{"pkg/b.go"}, core.ClaimSuperseded, jan, mar)
	oldFinding.Class = core.FindingClassConformance
	newFinding := claim("finding-new", "findings", "finding", "New finding.", []string{"pkg/b.go"}, core.ClaimFresh, mar, time.Time{})
	newFinding.Class = core.FindingClassConformance
	claims := []core.SemanticClaim{oldRule, newRule, oldFinding, newFinding}
	var links []core.ClaimLink
	for _, cl := range claims {
		links = append(links,
			core.ClaimLink{ClaimID: cl.ClaimID, Rel: core.RelCites, TargetRef: "docs/g.md#L1-L2", TargetHash: "h"},
			core.ClaimLink{ClaimID: cl.ClaimID, Rel: core.RelScopes, TargetRef: cl.Scope[0]})
	}
	if err := sq.PutClaims(ctx, claims, links); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	if err := sq.PutDocUnits(ctx, "svc", "x.md", []store.DocUnit{{UnitID: "u1", Path: "x.md", HeadingPath: "S", Kind: "section", Body: "b"}}); err != nil {
		t.Fatal(err)
	}
	oldLink := store.DocLink{TargetKind: "symbol", Target: "A", Line: 3, LinkTier: "exact", TargetHashAtLink: "h-a"}
	newLink := store.DocLink{TargetKind: "symbol", Target: "B", Line: 4, LinkTier: "exact", TargetHashAtLink: "h-b"}
	if err := sq.SyncDocLinksForUnit(ctx, "svc", "u1", []store.DocLink{oldLink}, jan.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err := sq.SyncDocLinksForUnit(ctx, "svc", "u1", []store.DocLink{newLink}, mar.UnixNano()); err != nil {
		t.Fatal(err)
	}

	return asOfSeed{
		srv:    NewServer(c),
		before: "2025-12-01T00:00:00Z",
		mid:    "2026-02-01T00:00:00Z",
		after:  "2026-04-01T00:00:00Z",
	}
}

// callToolFailure returns the JSON-RPC error a tool call answered with ("" when it succeeded).
func callToolFailure(t *testing.T, srv *Server, name string, args map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := srv.HandleRequest(context.Background(), raw)
	if !ok {
		t.Fatalf("%s: no response", name)
	}
	var env struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatal(err)
	}
	if env.Error == nil {
		return ""
	}
	return env.Error.Message
}

func asAnySlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// sameStrings compares in order: the verbs sort their own rows, so a helper
// that forgave order would hide a determinism regression.
func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestAsOfAcrossVerbs is GL-17's gate: `as_of` reaches every verb that reads a
// dated fact — obligations (SU-19), staleness_report, review_diff and
// findings_near — and each answers from the same validity windows, so a rule,
// a finding and a doc link that were replaced in March answer for February
// with what stood then and for April with what stands now. (memory_search,
// the row's original fourth verb, goes with anchored memories, SR-13.)
func TestAsOfAcrossVerbs(t *testing.T) {
	seed := newAsOfSeed(t)
	srv := seed.srv

	t.Run("obligations", func(t *testing.T) {
		for _, tc := range []struct {
			asOf string
			want []string
		}{{seed.before, nil}, {seed.mid, []string{"Old rule."}}, {seed.after, []string{"New rule."}}} {
			p := callTool(t, srv, "obligations", map[string]any{"service": "svc", "file": "pkg/a.go", "as_of": tc.asOf})
			var got []string
			for _, r := range asAnySlice(p["rules"]) {
				got = append(got, r.(map[string]any)["rule"].(string))
			}
			if !sameStrings(got, tc.want) {
				t.Errorf("obligations as of %s = %v, want %v", tc.asOf, got, tc.want)
			}
			if p["as_of"] != tc.asOf {
				t.Errorf("obligations as_of = %v, want %s", p["as_of"], tc.asOf)
			}
		}
	})

	t.Run("staleness_report", func(t *testing.T) {
		layerValid := func(p map[string]any, layer string) float64 {
			layers, _ := p["layers"].(map[string]any)
			l, _ := layers[layer].(map[string]any)
			n, _ := l["valid"].(float64)
			return n
		}
		linkTargets := func(p map[string]any) []string {
			var out []string
			for _, r := range asAnySlice(p["doc_links_as_of"]) {
				out = append(out, r.(map[string]any)["target"].(string))
			}
			return out
		}
		live := callTool(t, srv, "staleness_report", map[string]any{"service": "svc"})
		for _, k := range []string{"as_of", "as_of_scope", "doc_links_as_of", "layers_unavailable", "doc_links_unavailable"} {
			if _, ok := live[k]; ok {
				t.Errorf("a live staleness_report must not carry %q", k)
			}
		}
		mid := callTool(t, srv, "staleness_report", map[string]any{"service": "svc", "as_of": seed.mid})
		if mid["as_of"] != seed.mid || !strings.Contains(mid["as_of_scope"].(string), "index as it stands now") {
			t.Errorf("as_of/as_of_scope = %v / %v", mid["as_of"], mid["as_of_scope"])
		}
		if layerValid(mid, "obligations") != 1 || layerValid(mid, "findings") != 1 {
			t.Errorf("mid layers = %v, want one valid claim in obligations and findings (the since-superseded ones)", mid["layers"])
		}
		if got := linkTargets(mid); !sameStrings(got, []string{"A"}) {
			t.Errorf("mid doc links = %v, want [A]", got)
		}
		if _, ok := mid["doc_link_drift"]; ok {
			t.Error("a dated staleness_report must leave doc_link_drift out: drift is a comparison with now")
		}
		after := callTool(t, srv, "staleness_report", map[string]any{"service": "svc", "as_of": seed.after})
		if layerValid(after, "obligations") != 1 || layerValid(after, "findings") != 1 {
			t.Errorf("after layers = %v", after["layers"])
		}
		if got := linkTargets(after); !sameStrings(got, []string{"B"}) {
			t.Errorf("after doc links = %v, want [B]", got)
		}
		before := callTool(t, srv, "staleness_report", map[string]any{"service": "svc", "as_of": seed.before})
		if _, ok := before["layers"]; ok {
			t.Errorf("before anything: layers = %v, want none", before["layers"])
		}
		if n, _ := before["doc_links_as_of_count"].(float64); n != 0 {
			t.Errorf("before anything: doc_links_as_of_count = %v, want 0", before["doc_links_as_of_count"])
		}
	})

	t.Run("findings_near", func(t *testing.T) {
		ids := func(p map[string]any) []string {
			var out []string
			for _, r := range asAnySlice(p["findings"]) {
				out = append(out, r.(map[string]any)["claim_id"].(string))
			}
			return out
		}
		args := func(asOf string) map[string]any {
			a := map[string]any{"service": "svc", "subject": "pkg/b.go"}
			if asOf != "" {
				a["as_of"] = asOf
			}
			return a
		}
		if got := ids(callTool(t, srv, "findings_near", args(""))); !sameStrings(got, []string{"finding-new"}) {
			t.Errorf("live findings_near = %v, want only the current finding", got)
		}
		mid := callTool(t, srv, "findings_near", args(seed.mid))
		if got := ids(mid); !sameStrings(got, []string{"finding-old"}) {
			t.Errorf("findings_near as of mid = %v, want the finding that stood then, superseded since", got)
		}
		if mid["as_of"] != seed.mid {
			t.Errorf("findings_near as_of = %v", mid["as_of"])
		}
		if got := ids(callTool(t, srv, "findings_near", args(seed.after))); !sameStrings(got, []string{"finding-new"}) {
			t.Errorf("findings_near as of after = %v, want the current finding", got)
		}
		if got := ids(callTool(t, srv, "findings_near", args(seed.before))); len(got) != 0 {
			t.Errorf("findings_near before anything = %v, want none", got)
		}
	})

	t.Run("review_diff", func(t *testing.T) {
		rules := func(p map[string]any) []string {
			var out []string
			for _, d := range asAnySlice(p["dimensions"]) {
				dim := d.(map[string]any)
				if dim["name"] != "obligations" {
					continue
				}
				for _, f := range asAnySlice(dim["findings"]) {
					out = append(out, f.(map[string]any)["message"].(string))
				}
			}
			return out
		}
		mid := callTool(t, srv, "review_diff", map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "as_of": seed.mid})
		if got := rules(mid); !sameStrings(got, []string{"Old rule."}) {
			t.Errorf("review_diff as of mid: obligations = %v, want the rule that governed then", got)
		}
		if mid["as_of"] != seed.mid || mid["findings_persisted"] != false {
			t.Errorf("review_diff as_of = %v, findings_persisted = %v; a dated review echoes its date and persists nothing", mid["as_of"], mid["findings_persisted"])
		}
		after := callTool(t, srv, "review_diff", map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "as_of": seed.after})
		if got := rules(after); !sameStrings(got, []string{"New rule."}) {
			t.Errorf("review_diff as of after: obligations = %v, want the current rule", got)
		}
	})

	t.Run("a date that means nothing is refused, never answered as now", func(t *testing.T) {
		for _, tc := range []struct {
			tool string
			args map[string]any
		}{
			{"obligations", map[string]any{"service": "svc", "file": "pkg/a.go"}},
			{"staleness_report", map[string]any{"service": "svc"}},
			{"findings_near", map[string]any{"service": "svc", "subject": "pkg/b.go"}},
			{"review_diff", map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}}},
		} {
			tc.args["as_of"] = "last tuesday"
			if msg := callToolFailure(t, srv, tc.tool, tc.args); !strings.Contains(msg, tc.tool+":") || !strings.Contains(msg, "as_of") {
				t.Errorf("%s with as_of %q: error = %q, want a params error naming the tool and as_of", tc.tool, "last tuesday", msg)
			}
		}
	})
}

// TestAsOfFilter pins the resolver on its own: nil is now and changes
// nothing, and the window is half-open like core.SemanticClaim.ValidAt.
func TestAsOfFilter(t *testing.T) {
	var none *asOfFilter
	if none.active() || none.label() != "" || !none.instant().IsZero() {
		t.Error("the nil filter is 'now': inactive, no label, no instant")
	}
	in := []core.SemanticClaim{{ClaimID: "a"}}
	if got := none.claims(in); len(got) != 1 {
		t.Errorf("the nil filter kept %d claims, want all", len(got))
	}
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	f := &asOfFilter{at: mar}
	kept := f.claims([]core.SemanticClaim{
		{ClaimID: "closed-at-instant", ValidFrom: jan, ValidTo: mar},
		{ClaimID: "opens-at-instant", ValidFrom: mar},
		{ClaimID: "unconstrained"},
	})
	if len(kept) != 2 || kept[0].ClaimID != "opens-at-instant" || kept[1].ClaimID != "unconstrained" {
		t.Errorf("claims at the boundary = %+v, want the one opening at the instant and the unconstrained one", kept)
	}
	if f.label() != "2026-03-01T00:00:00Z" {
		t.Errorf("label = %q", f.label())
	}
}
