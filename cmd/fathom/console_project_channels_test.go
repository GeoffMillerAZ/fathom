package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func TestProjectWiring_ChannelHealthPreservesUnknownAndZero(t *testing.T) {
	zero := 0
	cost := 2.5
	channels := []doctorChannel{
		{Channel: "prime", State: "on but unused", Reason: "no payload rows", Injections: &zero, TokensPerDayEstimate: &cost, Coverage: "bounded log", CostGroup: "session-brief", LastSkipReason: "not ready"},
		{Channel: "guard", State: "on but not ready", Reason: "<unavailable>", Coverage: "hook log unavailable"},
	}
	page := renderProjectWiring(projectContext{svc: "fixture"}, console.WiringViewModel{}, nil, channels)
	for _, want := range []string{"project-channels", "prime", "on but unused", "no payload rows", "bounded log", "session-brief", "not ready", ">0<", ">2.5<", "guard", "on but not ready", "&lt;unavailable&gt;", "not measured", "hook log unavailable"} {
		if !strings.Contains(page, want) {
			t.Errorf("channel health table missing %q", want)
		}
	}
	if strings.Contains(page, "<unavailable>") {
		t.Error("channel reason was not escaped")
	}
	assertRoleUtilitiesOnly(t, page)
}
