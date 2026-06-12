package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

// templFixtureFull exercises all declaration forms and call-edge detection.
const templFixtureFull = `package views

import "github.com/example/app/layouts"

templ Page(title string) {
	@layouts.Base(title) {
		<h1>{ title }</h1>
		@Sidebar()
		@Sidebar()
	}
}

templ Sidebar() {
	<aside>sidebar</aside>
}

script initMap(lat, lng float64) {
	console.log(lat, lng);
}

css cardStyle() {
	background-color: white;
}

func helper(s string) string {
	return s
}
`

// templFixtureEmpty has only package and imports — no symbols.
const templFixtureEmpty = `package views

import "fmt"
`

// templFixtureOnlyPkg has only the package declaration.
const templFixtureOnlyPkg = `package views
`

func TestTemplExtractor_ComponentWithParams(t *testing.T) {
	x := extractor.NewTemplExtractor()
	nodes, err := x.Extract("svc", "web/views/page.templ", []byte(templFixtureFull))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("templ Page", func(t *testing.T) {
		n, ok := byName["Page"]
		if !ok {
			t.Fatal("expected symbol Page")
		}
		if n.Kind != core.KindTemplComponent {
			t.Errorf("Kind: got %q, want KindTemplComponent", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		if n.FilePath != "web/views/page.templ" {
			t.Errorf("FilePath: got %q, want %q", n.FilePath, "web/views/page.templ")
		}
		if n.Service != "svc" {
			t.Errorf("Service: got %q, want %q", n.Service, "svc")
		}
		if n.Signature == "" {
			t.Error("Signature: must not be empty")
		}
	})

	t.Run("templ Sidebar", func(t *testing.T) {
		n, ok := byName["Sidebar"]
		if !ok {
			t.Fatal("expected symbol Sidebar")
		}
		if n.Kind != core.KindTemplComponent {
			t.Errorf("Kind: got %q, want KindTemplComponent", n.Kind)
		}
	})
}

func TestTemplExtractor_ScriptAndCSS(t *testing.T) {
	x := extractor.NewTemplExtractor()
	nodes, err := x.Extract("svc", "web/views/page.templ", []byte(templFixtureFull))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("script initMap", func(t *testing.T) {
		n, ok := byName["initMap"]
		if !ok {
			t.Fatal("expected symbol initMap")
		}
		if n.Kind != core.KindTemplComponent {
			t.Errorf("Kind: got %q, want KindTemplComponent", n.Kind)
		}
	})

	t.Run("css cardStyle", func(t *testing.T) {
		n, ok := byName["cardStyle"]
		if !ok {
			t.Fatal("expected symbol cardStyle")
		}
		if n.Kind != core.KindTemplComponent {
			t.Errorf("Kind: got %q, want KindTemplComponent", n.Kind)
		}
	})
}

func TestTemplExtractor_PlainFunc(t *testing.T) {
	x := extractor.NewTemplExtractor()
	nodes, err := x.Extract("svc", "web/views/page.templ", []byte(templFixtureFull))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	n, ok := byName["helper"]
	if !ok {
		t.Fatal("expected symbol helper (plain Go func inside .templ file)")
	}
	if n.Kind != core.KindTemplComponent {
		t.Errorf("Kind: got %q, want KindTemplComponent", n.Kind)
	}
}

func TestTemplExtractor_CallEdges(t *testing.T) {
	x := extractor.NewTemplExtractor()
	nodes, err := x.Extract("svc", "web/views/page.templ", []byte(templFixtureFull))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("pkg-qualified call edge layouts.Base", func(t *testing.T) {
		page, ok := byName["Page"]
		if !ok {
			t.Fatal("symbol Page not found")
		}
		if !containsStr(page.Calls, "layouts.Base") {
			t.Errorf("Page.Calls = %v; want layouts.Base", page.Calls)
		}
	})

	t.Run("unqualified call edge Sidebar", func(t *testing.T) {
		page, ok := byName["Page"]
		if !ok {
			t.Fatal("symbol Page not found")
		}
		if !containsStr(page.Calls, "Sidebar") {
			t.Errorf("Page.Calls = %v; want Sidebar", page.Calls)
		}
	})

	t.Run("dedupe repeated @Sidebar() call", func(t *testing.T) {
		page, ok := byName["Page"]
		if !ok {
			t.Fatal("symbol Page not found")
		}
		count := 0
		for _, c := range page.Calls {
			if c == "Sidebar" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 Sidebar edge, got %d (Calls=%v)", count, page.Calls)
		}
	})
}

func TestTemplExtractor_EmptyFile(t *testing.T) {
	x := extractor.NewTemplExtractor()
	nodes, err := x.Extract("svc", "empty.templ", []byte(""))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("empty file: expected 0 nodes, got %d", len(nodes))
	}
}

func TestTemplExtractor_OnlyPackageAndImports(t *testing.T) {
	x := extractor.NewTemplExtractor()

	for name, fixture := range map[string]string{
		"only-pkg":         templFixtureOnlyPkg,
		"pkg-plus-imports": templFixtureEmpty,
	} {
		nodes, err := x.Extract("svc", "noop.templ", []byte(fixture))
		if err != nil {
			t.Fatalf("%s: Extract: %v", name, err)
		}
		if len(nodes) != 0 {
			t.Errorf("%s: expected 0 nodes, got %d", name, len(nodes))
		}
	}
}

func TestTemplExtractor_SourceOrder(t *testing.T) {
	x := extractor.NewTemplExtractor()
	nodes, err := x.Extract("svc", "web/views/page.templ", []byte(templFixtureFull))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Source order: Page, Sidebar, initMap, cardStyle, helper
	wantOrder := []string{"Page", "Sidebar", "initMap", "cardStyle", "helper"}
	if len(nodes) != len(wantOrder) {
		t.Fatalf("expected %d nodes, got %d: %v", len(wantOrder), len(nodes), nodeNames(nodes))
	}
	for i, want := range wantOrder {
		if nodes[i].Name != want {
			t.Errorf("nodes[%d]: got %q, want %q", i, nodes[i].Name, want)
		}
	}
}

func TestTemplExtractor_Determinism(t *testing.T) {
	x := extractor.NewTemplExtractor()
	n1, err := x.Extract("svc", "page.templ", []byte(templFixtureFull))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "page.templ", []byte(templFixtureFull))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, n1[i].Name, n2[i].Name)
		}
		if len(n1[i].Calls) != len(n2[i].Calls) {
			t.Errorf("non-deterministic calls at [%d]: %v vs %v", i, n1[i].Calls, n2[i].Calls)
		}
	}
}

// containsStr reports whether s is in slice.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// nodeNames returns a slice of node names for diagnostics.
func nodeNames(nodes []core.SymbolNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	return names
}
