package viz

import (
	"strings"
	"testing"
)

func tornadoFixture() []TornadoItem {
	return []TornadoItem{
		{Label: "habitability", Low: 3000, High: 9000, Prefix: "$"},
		{Label: "deposit", Low: 4000, High: 5000, Prefix: "$"},
		{Label: "fees", Low: 4200, High: 8000, Prefix: "$"},
	}
}

func TestTornado_Golden(t *testing.T) {
	// Baseline at 4500; sorted widest-swing first → habitability, fees, deposit.
	got := Tornado(tornadoFixture(), TornadoOptions{Baseline: 4500, AriaLabel: "what moves recovery"})
	assertGolden(t, got, "tornado_default")
}

func TestTornado_SortedBySwing(t *testing.T) {
	got := Tornado(tornadoFixture(), TornadoOptions{Baseline: 4500})
	// Widest swing (habitability, 6000) must appear before the narrowest (deposit, 1000).
	hi := strings.Index(got, "habitability")
	dep := strings.Index(got, "deposit")
	if hi < 0 || dep < 0 || hi > dep {
		t.Errorf("expected widest-swing factor first: habitability@%d deposit@%d\n%s", hi, dep, got)
	}
}

func TestTornado_Empty(t *testing.T) {
	got := Tornado(nil, TornadoOptions{})
	if !strings.Contains(got, "<svg") {
		t.Error("empty tornado missing svg")
	}
	if strings.Contains(got, "<rect") {
		t.Error("empty tornado should render no bars")
	}
}

func TestTornado_BidirectionalColoring(t *testing.T) {
	// A factor straddling the baseline gets both a danger (below) and positive (above) segment.
	got := Tornado([]TornadoItem{{Label: "x", Low: 0, High: 100}}, TornadoOptions{Baseline: 50})
	if !strings.Contains(got, cssColor("danger")) || !strings.Contains(got, cssColor("positive")) {
		t.Errorf("straddling factor should have both danger + positive segments: %s", got)
	}
}

func TestTornado_RoleOverrideAndSafe(t *testing.T) {
	got := Tornado([]TornadoItem{{Label: "x", Low: 80, High: 20, Role: "accent"}}, TornadoOptions{Baseline: 50})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") || strings.Contains(got, `width="-`) {
		t.Errorf("inverted/role item produced bad output: %s", got)
	}
	if !strings.Contains(got, cssColor("accent")) {
		t.Errorf("role override should be honored: %s", got)
	}
}

func TestTornado_Escaping(t *testing.T) {
	got := Tornado([]TornadoItem{{Label: `<script>`, Low: 0, High: 10}}, TornadoOptions{Baseline: 5})
	if strings.Contains(got, "<script>") {
		t.Errorf("tornado did not escape label: %s", got)
	}
}
