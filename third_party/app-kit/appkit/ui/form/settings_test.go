package form_test

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"golang.org/x/net/html"
)

func parseSettings(t *testing.T, body string) *html.Node {
	t.Helper()
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func settingsNodes(root *html.Node, element string) []*html.Node {
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == element {
			found = append(found, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func settingsAttr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func settingsText(n *html.Node) string {
	var out strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			out.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return out.String()
}

func classHas(n *html.Node, class string) bool {
	for _, got := range strings.Fields(settingsAttr(n, "class")) {
		if got == class {
			return true
		}
	}
	return false
}

func TestRenderSettingsSectionsSemanticStructure(t *testing.T) {
	firstID := `設定:"地域"<main>`
	longDescription := `Choose a region <script>alert("no")</script> & keep this deliberately long explanation readable.`
	trusted := template.HTML(`<label for="timezone">Time zone</label><select id="timezone"><option>UTC</option></select>`)
	body, err := form.RenderSettingsSections([]form.SettingsSection{
		{HeadingID: firstID, Title: `Region <primary> & locale`, Description: longDescription, Content: trusted},
		{HeadingID: "appearance:v2", Title: "Appearance", Content: template.HTML(`<span data-reviewed="true">System</span>`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := parseSettings(t, body)
	sections := settingsNodes(root, "section")
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(sections))
	}
	for i, want := range []struct {
		id, title string
		border    string
	}{{firstID, `Region <primary> & locale`, "border-y"}, {"appearance:v2", "Appearance", "border-b"}} {
		section := sections[i]
		if settingsAttr(section, "aria-labelledby") != want.id || !classHas(section, "sm:grid-cols-2") || !classHas(section, want.border) {
			t.Errorf("section %d association/classes changed: %#v", i, section.Attr)
		}
		headings := settingsNodes(section, "h3")
		if len(headings) != 1 || settingsAttr(headings[0], "id") != want.id || settingsText(headings[0]) != want.title {
			t.Errorf("section %d heading changed", i)
		}
	}
	if got := settingsText(settingsNodes(sections[0], "p")[0]); got != longDescription {
		t.Fatalf("description text = %q", got)
	}
	if len(settingsNodes(root, "script")) != 0 || len(settingsNodes(root, "select")) != 1 || !strings.Contains(body, string(trusted)) {
		t.Fatal("untrusted text executed or reviewed content was not preserved")
	}
	if len(settingsNodes(sections[1], "p")) != 0 {
		t.Fatal("empty optional description rendered a paragraph")
	}
}

func TestRenderSettingsSectionsValidatesAtomically(t *testing.T) {
	valid := form.SettingsSection{HeadingID: "first", Title: "First", Content: template.HTML(`<span>first content</span>`)}
	tests := []struct {
		name   string
		second form.SettingsSection
	}{
		{"empty id", form.SettingsSection{Title: "Second"}},
		{"blank title", form.SettingsSection{HeadingID: "second", Title: " \t\n"}},
		{"duplicate id", form.SettingsSection{HeadingID: "first", Title: "Second"}},
		{"unicode whitespace id", form.SettingsSection{HeadingID: "bad\u2003id", Title: "Second"}},
		{"control id", form.SettingsSection{HeadingID: "bad\u0001id", Title: "Second"}},
		{"invalid UTF-8 id", form.SettingsSection{HeadingID: "bad\xffid", Title: "Second"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := form.RenderSettingsSections([]form.SettingsSection{valid, tt.second})
			if err == nil || body != "" {
				t.Fatalf("invalid second section returned body=%q err=%v", body, err)
			}
			if !strings.Contains(err.Error(), "section 1") {
				t.Fatalf("error does not name section index: %v", err)
			}
			if tt.second.HeadingID != "" && strings.Contains(err.Error(), tt.second.HeadingID) {
				t.Fatalf("error reflected untrusted id: %v", err)
			}
		})
	}
}

func TestRenderSettingsSectionsEmptyAndNoRuntimeAuthority(t *testing.T) {
	body, err := form.RenderSettingsSections(nil)
	if err != nil || body != "" {
		t.Fatalf("empty list = %q, %v", body, err)
	}
	body, err = form.RenderSettingsSections([]form.SettingsSection{{HeadingID: "summary", Title: "Summary", Content: template.HTML(`<span>Read only</span>`)}})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"<form", "<script", " action=", "method=", "onclick=", "javascript:", "aria-live", "data-state"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("renderer invented runtime/form authority %q: %s", forbidden, body)
		}
	}
}

func ExampleRenderSettingsSections() {
	control := form.RenderInput(form.Input{ID: "display-name", Name: "display_name", Value: "Alex"})
	field := form.RenderField(form.Field{Label: "Display name", For: "display-name", Control: template.HTML(control)})
	body, err := form.RenderSettingsSections([]form.SettingsSection{{
		HeadingID: "personal-details", Title: "Personal details",
		Description: "Choose how your name appears.", Content: template.HTML(field),
	}})
	fmt.Println(err == nil, strings.Count(body, "<section"), strings.Contains(body, `name="display_name"`))
	// Output: true 1 true
}
