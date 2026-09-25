package gallery

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestProvenanceNarrowContentOwnsItsHorizontalOverflow(t *testing.T) {
	content, err := buildProvenance()
	if err != nil {
		t.Fatal(err)
	}
	const syntheticNotice = "All figures on this page are synthetic examples. Source labels and citations demonstrate the API; they do not substantiate these example values."
	if !strings.Contains(string(content), syntheticNotice) {
		t.Error("provenance examples lack the explicit synthetic-value notice")
	}
	if !strings.Contains(string(content), "carries a source line or an explicit mechanism waiver") {
		t.Error("page-level enforcement claim omits the mechanism waiver")
	}
	doc, err := html.Parse(strings.NewReader(string(content)))
	if err != nil {
		t.Fatal(err)
	}

	var viewports, examples, specimenGrids []*html.Node
	var finalSymbol *html.Node
	provenanceWalk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if n.Data == "div" && provenanceAttr(n, "role") == "region" && provenanceDescendant(n, "svg") != nil {
			viewports = append(viewports, n)
		}
		if n.Data == "pre" {
			examples = append(examples, n)
		}
		if n.Data == "div" && provenanceClasses(n, "grid", "lg:grid-cols-2") {
			specimenGrids = append(specimenGrids, n)
		}
		if n.Data == "code" && strings.Contains(provenanceText(n), "TestEveryGalleryFigureCarriesASource") {
			finalSymbol = n
		}
	})

	if len(viewports) != 5 {
		t.Fatalf("keyboard-scrollable visualization regions = %d, want 5", len(viewports))
	}
	for _, viewport := range viewports {
		if provenanceAttr(viewport, "tabindex") != "0" || provenanceAttr(viewport, "aria-label") == "" ||
			!provenanceClasses(viewport, "max-w-full", "overflow-x-auto", "focus-visible:outline-2", "focus-visible:outline-focus") {
			t.Errorf("visualization overflow is not a named keyboard region: %#v", viewport.Attr)
		}
	}

	if len(specimenGrids) != 5 {
		t.Fatalf("specimen grids = %d, want 5", len(specimenGrids))
	}
	for _, grid := range specimenGrids {
		children := provenanceElementChildren(grid)
		if len(children) != 2 || !provenanceClasses(children[0], "min-w-0") || !provenanceClasses(children[1], "min-w-0") {
			t.Errorf("specimen grid cells do not release intrinsic minimum width")
		}
	}

	if len(examples) != 6 {
		t.Fatalf("code examples = %d, want 6", len(examples))
	}
	labels := map[string]bool{}
	for _, example := range examples {
		label := provenanceAttr(example, "aria-label")
		labels[label] = true
		if provenanceAttr(example, "tabindex") != "0" || provenanceAttr(example, "role") != "region" || label == "" ||
			!provenanceClasses(example, "overflow-x-auto", "focus-visible:outline-2", "focus-visible:outline-focus") {
			t.Errorf("code example is not a named keyboard scroll region: %#v", example.Attr)
		}
	}
	if len(labels) != len(examples) {
		t.Error("code example region labels must identify their individual examples")
	}
	if finalSymbol == nil || !provenanceClasses(finalSymbol, "[overflow-wrap:anywhere]") {
		t.Error("long inline test symbol lacks an anywhere wrap opportunity")
	}
}

func provenanceWalk(n *html.Node, visit func(*html.Node)) {
	visit(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		provenanceWalk(child, visit)
	}
}

func provenanceAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func provenanceClasses(n *html.Node, wants ...string) bool {
	classes := strings.Fields(provenanceAttr(n, "class"))
	for _, want := range wants {
		found := false
		for _, class := range classes {
			found = found || class == want
		}
		if !found {
			return false
		}
	}
	return true
}

func provenanceDescendant(n *html.Node, element string) *html.Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == element {
			return child
		}
		if found := provenanceDescendant(child, element); found != nil {
			return found
		}
	}
	return nil
}

func provenanceElementChildren(n *html.Node) []*html.Node {
	var children []*html.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			children = append(children, child)
		}
	}
	return children
}

func provenanceText(n *html.Node) string {
	var b strings.Builder
	provenanceWalk(n, func(child *html.Node) {
		if child.Type == html.TextNode {
			b.WriteString(child.Data)
		}
	})
	return b.String()
}
