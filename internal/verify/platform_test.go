package verify

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// platformEvidence: an index holding only the symbols in have, over a corpus
// with an excluded (_windows.go) and an included Go file.
type platformEvidence struct {
	fakeGoneEvidence
	have map[string]bool
}

func (p platformEvidence) FindSymbol(s string) []core.SymbolNode {
	if !p.have[s] {
		return nil
	}
	return []core.SymbolNode{{Name: s, FilePath: "cmd/x/inc.go", Line: 3, Kind: core.KindGoFunc, Tier: core.TierVerified}}
}
func (platformEvidence) TestReaches(string) []string          { return []string{"TestX"} }
func (platformEvidence) Reachable(string, string) bool        { return true }
func (platformEvidence) Contradictions() []core.Contradiction { return nil }
func (platformEvidence) SpecImplementors(string) []string     { return nil }

func platformFixture() platformEvidence {
	ev := platformEvidence{have: map[string]bool{"Included": true}}
	ev.goFiles = []GoFile{
		{Path: "cmd/x/host_windows.go", Content: []byte("package main\n\ntype winHost struct{}\n\nfunc (h *winHost) ReadHost() {}\n\nfunc WinOnly() {}\n"), Excluded: "build-tag-excluded (go/build context mismatch)"},
		{Path: "cmd/x/inc.go", Content: []byte("package main\n\nfunc Included() {}\n")},
	}
	return ev
}

func TestPlatformExcluded_OperationalNotFailing(t *testing.T) {
	plan := Plan{Service: "svc", Tasks: []Task{
		{ID: "P-1", Claimed: true, Expect: Expect{Symbol: "winHost.ReadHost", Package: "cmd/x", ReachableFrom: "TestY"}},
		{ID: "P-2", Claimed: true, Expect: Expect{Symbol: "WinOnly"}},
		{ID: "P-3", Claimed: true, Expect: Expect{Symbol: "MissingEverywhere"}},
		{ID: "P-4", Claimed: true, Expect: Expect{Symbol: "Included"}},
	}}
	rep := VerifyPlan(plan, platformFixture())
	for _, id := range []string{"P-1", "P-2"} {
		var tv TaskVerdict
		for _, x := range rep.Tasks {
			if x.TaskID == id {
				tv = x
			}
		}
		if tv.Status != Unknown || tv.Label != PlatformExcludedLabel || !OperationalRow(tv) {
			t.Fatalf("%s: status %s label %s operational %v, want Unknown platform-excluded operational", id, tv.Status, tv.Label, OperationalRow(tv))
		}
		d := tv.Criteria[0].Detail
		if !strings.Contains(d, "platform: defined only in cmd/x/host_windows.go, excluded on ") {
			t.Errorf("%s detail %q", id, d)
		}
	}
	if tv := rep.Tasks[2]; tv.Status != Fail || OperationalRow(tv) {
		t.Errorf("missing everywhere: status %s, want Fail", tv.Status)
	}
	if tv := rep.Tasks[3]; tv.Status != Pass {
		t.Errorf("included: status %s, want Pass", tv.Status)
	}
	// Only P-1 and P-2 unproven operationally; P-3's failure keeps exit 1.
	if OperationalShortfall([]Report{rep}, 3, true) {
		t.Error("a genuine failure present: shortfall must not be operational")
	}
	only := Report{Tasks: rep.Tasks[:2]}
	if !OperationalShortfall([]Report{only}, 2, true) {
		t.Error("only platform rows unproven: want operational shortfall")
	}
}
