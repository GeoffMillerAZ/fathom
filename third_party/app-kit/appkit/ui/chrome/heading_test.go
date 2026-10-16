package chrome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	xhtml "golang.org/x/net/html"
)

func sampleHeading() PageHeading {
	return PageHeading{
		ID:          "page-title",
		Breadcrumbs: []Crumb{{Label: "Fleet", Href: "/fleet"}, {Label: "fathom", Href: "/projects/fathom"}, {Label: "Plans"}},
		Eyebrow:     "Project",
		Title:       "Plans for fathom",
		Meta:        []string{"12 proven", "3 open", "As of 12:03 UTC"},
		Actions: []form.Button{
			{Label: "Verify command", Href: "/verbs/verify", Variant: form.ButtonGhost},
			{Label: "Open ledger", Href: "/ledger"},
		},
	}
}

func headingGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".html.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 go test to generate)", path, err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden:\n got: %s\nwant: %s", name, got, want)
	}
}

func TestPageHeadingGolden(t *testing.T) {
	full, err := RenderPageHeading(sampleHeading())
	if err != nil {
		t.Fatal(err)
	}
	again, _ := RenderPageHeading(sampleHeading())
	if full != again {
		t.Fatal("same heading rendered differently")
	}
	headingGolden(t, "page_heading_full", full)
	minimal, err := RenderPageHeading(PageHeading{Title: "Settings"})
	if err != nil {
		t.Fatal(err)
	}
	headingGolden(t, "page_heading_minimal", minimal)
}

func headingNodes(t *testing.T, markup string) *xhtml.Node {
	t.Helper()
	doc, err := xhtml.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func headingFind(n *xhtml.Node, match func(*xhtml.Node) bool) []*xhtml.Node {
	var out []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func headingAttr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func phText(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func phTag(name string) func(*xhtml.Node) bool {
	return func(n *xhtml.Node) bool { return n.Data == name }
}

func TestPageHeadingStructureAndOrder(t *testing.T) {
	markup, err := RenderPageHeading(sampleHeading())
	if err != nil {
		t.Fatal(err)
	}
	doc := headingNodes(t, markup)
	h1s := headingFind(doc, phTag("h1"))
	if len(h1s) != 1 || phText(h1s[0]) != "Plans for fathom" || headingAttr(h1s[0], "id") != "page-title" {
		t.Fatalf("want exactly one titled h1, got %d", len(h1s))
	}
	if !strings.Contains(markup, Breadcrumbs(sampleHeading().Breadcrumbs)) {
		t.Error("breadcrumbs must be the shared Breadcrumbs output unchanged")
	}
	for _, a := range sampleHeading().Actions {
		if !strings.Contains(markup, form.RenderButton(a)) {
			t.Errorf("action %q must be the shared RenderButton output unchanged", a.Label)
		}
	}
	// DOM order is reading order: trail, eyebrow, title, meta, actions.
	order := []string{`aria-label="Breadcrumb"`, ">Project<", ">Plans for fathom<", ">12 proven<", ">Verify command<"}
	last := -1
	for _, piece := range order {
		i := strings.Index(markup, piece)
		if i <= last {
			t.Fatalf("%q out of reading order", piece)
		}
		last = i
	}
	meta := headingFind(doc, phTag("ul"))
	if len(meta) != 1 || len(headingFind(meta[0], phTag("li"))) != 3 {
		t.Fatal("meta must be one list of items")
	}
	row := h1s[0].Parent.Parent
	cls := " " + headingAttr(row, "class") + " "
	for _, want := range []string{" flex-col ", " sm:flex-row ", " sm:justify-between "} {
		if !strings.Contains(cls, want) {
			t.Errorf("title/actions row lacks %q: actions must wrap below at narrow width", want)
		}
	}
	actions := row.LastChild
	acls := " " + headingAttr(actions, "class") + " "
	for _, want := range []string{" flex-wrap ", " *:min-h-11 ", " sm:shrink-0 "} {
		if !strings.Contains(acls, want) {
			t.Errorf("actions container lacks %q", want)
		}
	}
	if !strings.Contains(headingAttr(h1s[0].Parent, "class"), "min-w-0") || !strings.Contains(headingAttr(h1s[0], "class"), "[overflow-wrap:anywhere]") {
		t.Error("long titles must shrink and wrap instead of overflowing")
	}
}

func TestPageHeadingOmitsEmptyParts(t *testing.T) {
	markup, err := RenderPageHeading(PageHeading{Title: "Settings"})
	if err != nil {
		t.Fatal(err)
	}
	doc := headingNodes(t, markup)
	for _, absent := range []string{"nav", "ul", "a", "button", "p"} {
		if len(headingFind(doc, phTag(absent))) != 0 {
			t.Errorf("minimal heading rendered an empty %s", absent)
		}
	}
	if strings.Contains(markup, " id=") || strings.Contains(markup, "mt-1") {
		t.Error("minimal heading must not invent an id or eyebrow spacing")
	}
}

func TestPageHeadingEscapesAndDisablesUnsafeActions(t *testing.T) {
	h := sampleHeading()
	h.Title = `<script>alert(1)</script>`
	h.Eyebrow = `"><img src=x onerror=alert(1)>`
	h.Meta = []string{`<b>bold</b>`}
	h.Breadcrumbs = []Crumb{{Label: `<i>crumb</i>`, Href: "/x"}, {Label: "here"}}
	h.Actions = []form.Button{{Label: "Run", Href: "javascript:alert(1)"}, {Label: "Save", Type: "button"}}
	markup, err := RenderPageHeading(h)
	if err != nil {
		t.Fatal(err)
	}
	doc := headingNodes(t, markup)
	for _, bad := range []string{"script", "img", "b", "i"} {
		if len(headingFind(doc, phTag(bad))) != 0 {
			t.Errorf("hostile text became a %s element", bad)
		}
	}
	if strings.Contains(markup, "javascript:") {
		t.Error("unsafe action destination survived")
	}
	inert := headingFind(doc, func(n *xhtml.Node) bool { return headingAttr(n, "aria-disabled") == "true" })
	if len(inert) != 1 || phText(inert[0]) != "Run" {
		t.Error("unsafe link must render as the shared inert label")
	}
	buttons := headingFind(doc, phTag("button"))
	if len(buttons) != 1 || headingAttr(buttons[0], "type") != "button" {
		t.Error("script-owned action keeps its native button type")
	}
}

func TestPageHeadingRefusesInvalidInputBeforeOutput(t *testing.T) {
	cases := map[string]func(*PageHeading){
		"blank title":        func(h *PageHeading) { h.Title = " " },
		"control title":      func(h *PageHeading) { h.Title = "a\tb" },
		"invalid utf8 title": func(h *PageHeading) { h.Title = "\xff" },
		"long title":         func(h *PageHeading) { h.Title = strings.Repeat("x", maxHeadingRunes+1) },
		"blank eyebrow":      func(h *PageHeading) { h.Eyebrow = "  " },
		"blank meta":         func(h *PageHeading) { h.Meta = []string{"ok", ""} },
		"too many meta":      func(h *PageHeading) { h.Meta = strings.Fields(strings.Repeat("m ", MaxHeadingMeta+1)) },
		"too many actions": func(h *PageHeading) {
			h.Actions = make([]form.Button, MaxHeadingActions+1)
			for i := range h.Actions {
				h.Actions[i].Label = "a"
			}
		},
		"blank action": func(h *PageHeading) { h.Actions = []form.Button{{Label: " "}} },
		"blank crumb":  func(h *PageHeading) { h.Breadcrumbs = []Crumb{{Label: ""}} },
		// An action with an unsafe destination renders inert through RenderButton;
		// Breadcrumbs writes hrefs as links, so the heading refuses them instead.
		"script crumb": func(h *PageHeading) {
			h.Breadcrumbs = []Crumb{{Label: "Fleet", Href: "javascript:alert(document.cookie)"}, {Label: "Plans"}}
		},
		"data crumb": func(h *PageHeading) {
			h.Breadcrumbs = []Crumb{{Label: "Fleet", Href: "data:text/html,<script>alert(1)</script>"}, {Label: "Plans"}}
		},
		"protocol-relative crumb": func(h *PageHeading) {
			h.Breadcrumbs = []Crumb{{Label: "Fleet", Href: "//evil.example/"}, {Label: "Plans"}}
		},
		"padded crumb": func(h *PageHeading) { h.Breadcrumbs = []Crumb{{Label: "Fleet", Href: " /fleet"}, {Label: "Plans"}} },
		"unsafe current crumb": func(h *PageHeading) {
			h.Breadcrumbs = []Crumb{{Label: "Fleet", Href: "/fleet"}, {Label: "Plans", Href: "javascript:x"}}
		},
		"id with space":    func(h *PageHeading) { h.ID = "page title" },
		"id with brackets": func(h *PageHeading) { h.ID = `x"><script>` },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			h := sampleHeading()
			change(&h)
			if out, err := RenderPageHeading(h); err == nil || out != "" {
				t.Fatalf("accepted invalid heading: %q", out)
			}
		})
	}
}

func ExampleRenderPageHeading() {
	heading, err := RenderPageHeading(PageHeading{
		Breadcrumbs: []Crumb{{Label: "Fleet", Href: "/fleet"}, {Label: "fathom"}},
		Eyebrow:     "Project",
		Title:       "fathom",
		Meta:        []string{"12 plans", "As of 12:03 UTC"},
		Actions:     []form.Button{{Label: "Open ledger", Href: "/ledger", Variant: form.ButtonGhost}},
	})
	if err != nil {
		return // refuse the page rather than render a partial heading
	}
	_ = heading
}
