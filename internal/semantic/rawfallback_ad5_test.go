package semantic

import (
	"fmt"
	"github.com/geoffmilleraz/fathom/internal/adr"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestAD5RawRowsInForce(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	for _, tt := range []struct {
		name, path, body string
		want             int
	}{
		{"accepted", "ADR.md", "## ADR-001: Current\n\n**Decision.** Always sort outputs.\n", 1},
		{"proposed", "ADR.md", "## ADR-001: Proposal\n\n**Status.** Proposed.\n\n**Decision.** Always sort outputs.\n", 0},
		{"rejected", "ADR.md", "## ADR-001: Rejected\n\n**Status.** Rejected.\n\n**Decision.** Always sort outputs.\n", 0},
		{"deferred", "ADR.md", "## ADR-001: Deferred\n\n**Status.** Deferred.\n\n**Decision.** Always sort outputs.\n", 0},
		{"superseded", "ADR.md", "## ADR-001: Old\n\n**Status.** Superseded by ADR-002.\n\n**Decision.** Always sort outputs.\n", 0},
		{"partly superseded", "ADR.md", "## ADR-001: Partial\n\n**Status.** Superseded in part by ADR-002.\n\n**Decision.** Always sort outputs.\n", 1},
		{"accepted mentions deferred", "ADR.md", "## ADR-001: Current\n\n**Status.** Accepted (the rejected alternative remains deferred).\n\n**Decision.** Always sort outputs.\n", 1},
		{"status colon", "ADR.md", "## ADR-001: Current\n\n**Status**: Proposed.\n\n**Decision.** Always sort outputs.\n", 0},
		{"archive", "docs/plans/archive/old.md", "# Plan\n\n- [ ] Always sort outputs.\n", 0},
		{"checked plan", "docs/plans/current.md", "# Plan\n\n- [x] Always sort outputs.\n", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			corpus := []core.SourceFile{{Path: tt.path, Content: []byte(tt.body)}}
			var stats RawFallbackStats
			rows, err := RawFallbackRules(nil, layer, corpus, "pkg/a.go", 2, "coverage gap", nil, nil, &stats)
			if err != nil || len(rows) != tt.want {
				t.Fatalf("rows = %+v, err = %v, want %d", rows, err, tt.want)
			}
			if tt.want == 0 && stats.InForceWithheld != 1 {
				t.Errorf("in-force withholds=%d, want 1", stats.InForceWithheld)
			}
			for _, row := range rows {
				if tt.name == "partly superseded" && row.InForce != partlySuperseded {
					t.Errorf("partial tag = %q", row.InForce)
				}
				if !strings.HasPrefix(row.Src, tt.path+"#L") {
					t.Errorf("citation = %q, want source document", row.Src)
				}
			}
		})
	}
}

func TestAD5RawRowsWithholdUnextractableSections(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	corpus := []core.SourceFile{{Path: "AGENTS.md", Content: []byte("# Heading only\n\n# Current instructions\n\nAlways sort outputs.\n")}}
	for _, root := range []bool{false, true} {
		var rows []Obligation
		var err error
		if root {
			rows, err = RootRawFallbackRules(nil, layer, corpus, 1, "coverage gap")
		} else {
			rows, err = RawFallbackRules(nil, layer, corpus, "pkg/a.go", 1, "coverage gap", nil, nil)
		}
		if err != nil || len(rows) != 1 || rows[0].Rule != "Always sort outputs." {
			t.Errorf("root=%v: rows = %+v, err = %v, want an instruction after withholding the heading", root, rows, err)
		}
	}
}

func TestAD5ClaimStandingFirstWord(t *testing.T) {
	for _, tt := range []struct{ status, want string }{
		{"Accepted (a proposed alternative is deferred).", inForce},
		{"Proposed, accepted only after review.", "proposed"},
		{"Rejected.", "rejected"}, {"Deferred.", "deferred"},
		{"Superseded by ADR-002.", fullySuperseded},
		{"Partly superseded by ADR-002.", partlySuperseded},
	} {
		t.Run(tt.status, func(t *testing.T) {
			body := "## ADR-001: Storage\n\n**Status**: " + tt.status + "\n\n**Decision.** Always sort outputs.\n"
			got, _ := claimInForce("ADR.md#L5-L5", adr.ParseSections([]byte(body)))
			if got != tt.want {
				t.Errorf("standing = %q, want %q", got, tt.want)
			}
		})
	}
}
func TestAD5CheckedCompiledPlanWithheld(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docs", "plans", "current.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	body := "# Plan\n\n- [x] Always sort outputs.\n  Never lose a completed row continuation.\n\n- [ ] Run the remaining check.\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	resolve := inForceResolver(root)
	for _, tt := range []struct{ cite, want string }{
		{"docs/plans/archive/old.md#L1-L2", "archived"},
		{"docs/plans/current.md#L3-L3", "checked"},
		{"docs/plans/current.md#L4-L4", "checked"},
		{"docs/plans/current.md#L6-L6", inForce},
	} {
		if got, _ := resolve(tt.cite); got != tt.want {
			t.Errorf("%s standing=%q want %q", tt.cite, got, tt.want)
		}
	}
}
func TestAD5RealCorpusStanding(t *testing.T) {
	for _, id := range []string{"ADR-056", "ADR-058", "ADR-061"} {
		data := h1LoadDoc(t, "ADR.md").Content
		sections := adr.ParseSections(data)
		found := false
		for _, sec := range sections {
			if sec.ID == id {
				found = true
				if got, _ := claimInForce(fmt.Sprintf("ADR.md#L%d-L%d", sec.Line, sec.Line), sections); got != inForce {
					t.Errorf("%s standing=%q", id, got)
				}
			}
		}
		if !found {
			t.Fatal(id)
		}
	}
}

func TestAD5RefindReportsInactiveStanding(t *testing.T) {
	data := []byte("## ADR-001: Storage\n\n**Status.** Proposed.\n\n**Decision.** Always sort outputs.\n")
	sec := SplitMarkdownSections("ADR.md", data)[0]
	row := Obligation{Subject: sec.Subject, Src: "ADR.md#L1-L6", Tier: "raw"}
	got, ok := NewRawRuleRefinder().Refind(row, data, "pkg/a.go", nil)
	if ok || got.InForce != "proposed" {
		t.Fatalf("refind inactive = %+v, %v; want proposed rejection", got, ok)
	}
}

func TestAD5RefindKeepsExtraTokens(t *testing.T) {
	body := "# Rules\nAlways retain evidence.\n" + strings.Repeat("Background material.\n", 12) + "Never bypass SpecialBoundary.\n" + strings.Repeat("Background material.\n", 8)
	sec := SplitMarkdownSections("AGENTS.md", []byte(body))[0]
	row := Obligation{Subject: sec.Subject, Src: "AGENTS.md#L1-L24", Tier: "raw"}
	wantStart, wantEnd := citeWindow(sec, append(pathRelevanceTokens("pkg/a.go"), symbolRelevanceTokens([]string{"SpecialBoundary"})...))
	got, ok := RefindRawRule(row, []byte(body), "pkg/a.go", []string{"SpecialBoundary"})
	want := fmt.Sprintf("AGENTS.md#L%d-L%d", wantStart, wantEnd)
	if !ok || got.Src != want || wantStart <= 1 {
		t.Fatalf("citation %s want %s, ok=%v", got.Src, want, ok)
	}
}
