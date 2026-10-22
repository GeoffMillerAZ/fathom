package console

import (
	"strings"
	"testing"
)

func TestWiringViewCountsUncertainDoctorRows(t *testing.T) {
	m := WiringView(WiringFacts{Checks: []WiringCheck{{Status: "unknown"}, {Status: "timeout"}, {Status: "skipped"}}})
	if m.Skip.V != 3 || !strings.Contains(m.Skip.Denominator, "1 unknown") || !strings.Contains(m.Skip.Denominator, "1 timeout") {
		t.Fatalf("uncertain rows disappeared: %+v", m.Skip)
	}
}
