package capability

import (
	"strings"
	"testing"
)

func TestCapabilityObligationsMissingModelRemedies(t *testing.T) {
	c, _ := Default().Get("obligations")
	for _, hooks := range []bool{false, true} {
		p := Probe{ObligationRules: 1, HooksReferenced: hooks, ModelUnavailable: "fixture daemon unreachable"}
		state, remedy := c.Detect(p), c.Remedy(p)
		if !hooks {
			if !state.Broken || state.Unknown != "" || remedy.Cmd != "fathom init --hooks" {
				t.Errorf("missing hooks must be a local failure: %+v %+v", state, remedy)
			}
		} else if state.Unknown == "" || state.Broken || remedy.Cmd != "" || !strings.Contains(remedy.Note, p.ModelUnavailable) {
			t.Errorf("missing model must suggest reachability, never compile: %+v %+v", state, remedy)
		}
	}
}

func TestCapabilityIndexMissingModelRemedy(t *testing.T) {
	c, _ := Default().Get("index")
	for _, p := range []Probe{
		{ModelUnavailable: "fixture daemon unreachable"},
		{ModelUnavailable: "fixture daemon unreachable", IndexedSymbols: 1, IndexStale: true},
	} {
		state, remedy := c.Detect(p), c.Remedy(p)
		if state.Unknown == "" || state.Broken || remedy.Cmd != "" || !strings.Contains(remedy.Note, p.ModelUnavailable) || !strings.Contains(remedy.Note, "reachability") {
			t.Errorf("missing model must suggest reachability, never init or ingest: %+v %+v", state, remedy)
		}
	}
}

func TestCapabilityObligationsUnindexedRepository(t *testing.T) {
	c, _ := Default().Get("obligations")
	p := Probe{ObligationRules: 1, HooksReferenced: true}
	state, remedy := c.Detect(p), c.Remedy(p)
	if state.Operating || state.Broken || state.Unknown != "" || state.Reason != "rules are authored but this repository is not indexed" || remedy.Cmd != "fathom init" {
		t.Fatalf("unindexed rules must be off with an init remedy: %+v %+v", state, remedy)
	}
}
