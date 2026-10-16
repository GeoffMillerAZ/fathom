package main

// primer_surface_test.go: the SessionStart and SubagentStart primers hold
// the stanza's promise (TestStanzaNamesOnlyRegisteredSurfaceTools). A
// session registered at /rpc/agent is told to call only what that surface
// serves, so neither primer cites a tool beyond the default eight, and the
// subagent primer for an indexed service fits its byte cap without the cut.

import (
	"strings"
	"testing"
)

func TestPrimersNameOnlyDefaultSurfaceTools(t *testing.T) {
	srv := fakeDaemon()
	defer srv.Close()
	off := offDefaultSurfaceTools()

	sub := subagentPrimer(srv.URL, "fathom", "", "")
	if sub == "" {
		t.Fatal("subagentPrimer returned \"\": fakeDaemon must resolve \"fathom\" as an indexed service")
	}
	session := buildSessionContext(srv.URL, "fathom")
	for label, text := range map[string]string{"subagent primer": sub, "SessionStart primer": session} {
		if cited := stanzaToolCitations(text, off); len(cited) > 0 {
			t.Errorf("%s cites %v, served only off the default /rpc/agent surface:\n%s", label, cited, text)
		}
	}
	if strings.HasSuffix(sub, subagentPrimerCut) || len(sub) > subagentPrimerBudget {
		t.Errorf("subagent primer is %d bytes or cut; the cap is %d:\n%s", len(sub), subagentPrimerBudget, sub)
	}
}
