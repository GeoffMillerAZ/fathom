package topology

import (
	"strings"
	"testing"
)

// sampleTree is a small global → region → cluster → workload hierarchy used
// across tests.
func sampleTree() []Node {
	return []Node{{
		Name:   "global",
		Status: Healthy,
		Children: []Node{{
			Name:   "us-east-1",
			Status: Degraded,
			Metric: "p99 142ms",
			Children: []Node{{
				Name:   "cluster-a",
				Status: Failed,
				Href:   "/clusters/a",
				Children: []Node{
					{Name: "checkout-svc", Status: Failed, Metric: "3/4 pods"},
					{Name: "cart-svc", Status: Healthy},
				},
			}},
		}},
	}}
}

func TestRender_StatusRoles(t *testing.T) {
	cases := []struct {
		status Status
		role   string // the badge background role utility we expect to see
	}{
		{Healthy, "bg-positive-soft"},
		{Degraded, "bg-caution-soft"},
		{Failed, "bg-danger-soft"},
		{Unknown, "bg-surface-sunken"}, // neutral pill, never a colored status
	}
	for _, c := range cases {
		got := Render([]Node{{Name: "n", Status: c.status}}, Options{})
		if !strings.Contains(got, c.role) {
			t.Errorf("status %q: want badge role utility %q in:\n%s", c.status, c.role, got)
		}
		if !strings.Contains(got, ">"+statusLabel(c.status)+"<") {
			t.Errorf("status %q: badge label %q missing", c.status, statusLabel(c.status))
		}
	}
}

func TestRender_ZeroStatusIsUnknown(t *testing.T) {
	// An unset Status must read as "unknown" (neutral), never as healthy/green.
	got := Render([]Node{{Name: "n"}}, Options{})
	if !strings.Contains(got, "bg-surface-sunken") || !strings.Contains(got, ">unknown<") {
		t.Errorf("zero status should render a neutral unknown badge:\n%s", got)
	}
	if strings.Contains(got, "bg-positive-soft") {
		t.Errorf("zero status must not render as healthy/green:\n%s", got)
	}
}

func TestRender_UnrecognizedStatusFallsBack(t *testing.T) {
	got := Render([]Node{{Name: "n", Status: Status("bogus")}}, Options{})
	if !strings.Contains(got, ">unknown<") || !strings.Contains(got, "bg-surface-sunken") {
		t.Errorf("unrecognized status should fall back to unknown:\n%s", got)
	}
}

func TestRender_NestingAndCollapse(t *testing.T) {
	got := Render(sampleTree(), Options{})
	// A node with children is a <details> group; a leaf is a treeitem div.
	if !strings.Contains(got, "<details") || !strings.Contains(got, "<summary") {
		t.Errorf("nested nodes should use no-JS <details>/<summary>:\n%s", got)
	}
	if !strings.Contains(got, `role="tree"`) || !strings.Contains(got, `role="treeitem"`) {
		t.Errorf("tree/treeitem roles missing:\n%s", got)
	}
	// Default (not Collapsed) renders groups open.
	if !strings.Contains(got, "<details role=\"treeitem\" class=\"border-t border-line first:border-t-0\" open>") {
		t.Errorf("default tree should render groups open:\n%s", got)
	}
	// Every level's names should be present (full hierarchy rendered).
	for _, name := range []string{"global", "us-east-1", "cluster-a", "checkout-svc", "cart-svc"} {
		if !strings.Contains(got, ">"+name+"<") {
			t.Errorf("node %q missing from tree:\n%s", name, got)
		}
	}
}

func TestRender_CollapsedOption(t *testing.T) {
	got := Render(sampleTree(), Options{Collapsed: true})
	if strings.Contains(got, " open>") {
		t.Errorf("Collapsed option should omit the open attribute:\n%s", got)
	}
	// Still a <details> disclosure (collapsible), just closed.
	if !strings.Contains(got, "<details") {
		t.Errorf("Collapsed tree should still use <details>:\n%s", got)
	}
}

func TestRender_NoJS(t *testing.T) {
	// Loopback Invariant (D5): no script tags, no on* handlers, no javascript:.
	got := Render(sampleTree(), Options{})
	for _, bad := range []string{"<script", "onclick", "onload", "javascript:"} {
		if strings.Contains(strings.ToLower(got), bad) {
			t.Errorf("tree must be JS-free (D5); found %q:\n%s", bad, got)
		}
	}
}

func TestRender_DrillThroughHref(t *testing.T) {
	got := Render([]Node{{Name: "svc", Href: "/clusters/a", Status: Healthy}}, Options{})
	if !strings.Contains(got, `<a href="/clusters/a"`) {
		t.Errorf("drill-through should render an anchor with the href:\n%s", got)
	}
	// No href → plain span, no anchor for the name.
	plain := Render([]Node{{Name: "svc", Status: Healthy}}, Options{})
	if strings.Contains(plain, "<a href=") {
		t.Errorf("node without Href should not render an anchor:\n%s", plain)
	}
}

func TestRender_Metric(t *testing.T) {
	got := Render([]Node{{Name: "svc", Status: Healthy, Metric: "p99 142ms"}}, Options{})
	if !strings.Contains(got, "p99 142ms") {
		t.Errorf("metric not rendered:\n%s", got)
	}
	// No metric → no trailing metric span.
	none := Render([]Node{{Name: "svc", Status: Healthy}}, Options{})
	if strings.Contains(none, "ml-auto") {
		t.Errorf("node without metric should not render the metric span:\n%s", none)
	}
}

func TestRender_Escaping(t *testing.T) {
	got := Render([]Node{{
		Name:   `<script>n</script>`,
		Status: Healthy,
		Metric: `<img src=x>`,
		Href:   `" onmouseover="alert(1)`,
		Children: []Node{{
			Name:   `<b>child</b>`,
			Status: Failed,
		}},
	}}, Options{AriaLabel: `<x>`})
	for _, bad := range []string{"<script>n", "<img src=x>", "<b>child</b>", `onmouseover="alert`} {
		if strings.Contains(got, bad) {
			t.Errorf("tree did not escape %q:\n%s", bad, got)
		}
	}
	// The escaped href attribute must not break out of the quoted value.
	if !strings.Contains(got, `&#34; onmouseover=`) {
		t.Errorf("href not escaped as an attribute value:\n%s", got)
	}
	// AriaLabel is escaped too.
	if strings.Contains(got, `aria-label="<x>"`) {
		t.Errorf("aria-label not escaped:\n%s", got)
	}
}

func TestRender_EmptyAndDefaults(t *testing.T) {
	got := Render(nil, Options{})
	if !strings.Contains(got, "No nodes.") {
		t.Errorf("empty tree should render an empty-state:\n%s", got)
	}
	if !strings.Contains(got, `aria-label="Topology"`) {
		t.Errorf("default aria-label should be Topology:\n%s", got)
	}
	// Token-only: no literal hex colors anywhere.
	for _, tree := range []string{got, Render(sampleTree(), Options{})} {
		if strings.Contains(tree, "#") {
			t.Errorf("tree must use token roles, not literal colors:\n%s", tree)
		}
	}
}

func TestRender_Indentation(t *testing.T) {
	// Deeper nodes get larger left padding so the hierarchy reads visually.
	got := Render(sampleTree(), Options{})
	for _, want := range []string{
		"padding-left:0.75rem", // depth 0 (global)
		"padding-left:1.75rem", // depth 1 (us-east-1)
		"padding-left:2.75rem", // depth 2 (cluster-a)
		"padding-left:3.75rem", // depth 3 (leaf workloads)
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing indent %q for nesting depth:\n%s", want, got)
		}
	}
}

func TestRender_Deterministic(t *testing.T) {
	a := Render(sampleTree(), Options{})
	b := Render(sampleTree(), Options{})
	if a != b {
		t.Errorf("topology render non-deterministic")
	}
	// Child order is preserved as given (no map iteration).
	got := Render([]Node{{Name: "root", Children: []Node{
		{Name: "z"}, {Name: "a"}, {Name: "m"},
	}}}, Options{})
	zi, ai, mi := strings.Index(got, ">z<"), strings.Index(got, ">a<"), strings.Index(got, ">m<")
	if !(zi < ai && ai < mi) {
		t.Errorf("child order not preserved (z,a,m); indices z=%d a=%d m=%d", zi, ai, mi)
	}
}
