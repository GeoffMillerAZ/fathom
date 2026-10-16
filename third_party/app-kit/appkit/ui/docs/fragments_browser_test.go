package docs_test

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"
	"testing"

	kitdocs "github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/docs"
)

// TestExportFragmentBrowserFixture is opt-in test tooling, not a static-site API.
// The browser probe consumes these actual renderer bytes with JavaScript disabled.
func TestExportFragmentBrowserFixture(t *testing.T) {
	output := os.Getenv("APPKIT_FRAGMENT_FIXTURE")
	if output == "" {
		t.Skip("set APPKIT_FRAGMENT_FIXTURE to an exclusive private JSON output file")
	}
	type fixture struct {
		Name string `json:"name"`
		ID   string `json:"id"`
		HTML string `json:"html"`
	}
	renderers := []struct {
		name   string
		render func(kitdocs.Heading) string
	}{
		{"boxed", func(h kitdocs.Heading) string { return kitdocs.RenderToC([]kitdocs.Heading{h}) }},
		{"panel", func(h kitdocs.Heading) string { return kitdocs.RenderToCPanel([]kitdocs.Heading{h}, kitdocs.Options{}) }},
		{"rail", func(h kitdocs.Heading) string { return kitdocs.RenderToCRail([]kitdocs.Heading{h}, kitdocs.Options{}) }},
		{"permalink", func(h kitdocs.Heading) string { return kitdocs.RenderPermalink(h.ID) }},
	}
	ids := []struct{ id, decoy string }{
		{"stable-section", "other"},
		{"path%2Fone", "path/one"},
		{"literal%25value", "literal%value"},
		{"資料-🧭", "other"},
		{"notes#appendix", "other"},
		{`quotes"-&-<detail>`, "other"},
	}
	var fixtures []fixture
	for _, renderer := range renderers {
		for _, target := range ids {
			var b strings.Builder
			b.WriteString(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Literal heading navigation</title><main><h1>Literal heading navigation</h1><p>Follow the section link using the keyboard.</p>`)
			b.WriteString(renderer.render(kitdocs.Heading{Level: 2, Text: "Requested section", ID: target.id}))
			fmt.Fprintf(&b, `<h2 id="%s" tabindex="-1">Different section: not the requested target</h2><p>Decoding the caller's ID a second time must not select this section.</p><h2 id="%s" tabindex="-1">Requested section</h2><p>The literal caller-owned ID must receive focus.</p></main></html>`,
				html.EscapeString(target.decoy), html.EscapeString(target.id))
			fixtures = append(fixtures, fixture{renderer.name + "/" + target.id, target.id, b.String()})
		}
	}
	f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(f).Encode(fixtures); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
