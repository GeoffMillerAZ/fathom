package form_test

import (
	"html/template"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"golang.org/x/net/html"
)

func TestSettingsControlColumnWrapsLongTextWithoutRewritingControls(t *testing.T) {
	label := strings.Repeat("WorkspaceIdentifier", 8)
	message := strings.Repeat("PreferenceReference", 8)
	for _, invalid := range []bool{false, true} {
		control := form.RenderInput(form.Input{ID: "workspace", Name: "workspace", Value: "Synthetic", HasHelp: !invalid, Invalid: invalid})
		field := form.Field{Label: label, For: "workspace", Control: template.HTML(control)}
		if invalid {
			field.Error = message
		} else {
			field.Help = message
		}
		content := form.RenderField(field) + form.RenderCheckbox(form.Checkbox{ID: "digest", Name: "digest", Label: label}) +
			form.RenderRadioGroup(form.RadioGroup{Name: "delivery", Legend: label, Help: message, Options: []form.Option{{Value: "daily", Label: label}}})
		body, err := form.RenderSettingsSections([]form.SettingsSection{{HeadingID: "preferences", Title: "Preferences", Description: "Synthetic preferences", Content: template.HTML(content)}})
		if err != nil {
			t.Fatal(err)
		}
		sections := settingsNodes(parseSettings(t, body), "section")
		if len(sections) != 1 {
			t.Fatalf("sections = %d", len(sections))
		}
		var columns []*html.Node
		for child := sections[0].FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "div" {
				columns = append(columns, child)
			}
		}
		if len(columns) != 2 || !classHas(columns[1], "min-w-0") || !classHas(columns[1], "[overflow-wrap:anywhere]") {
			t.Fatalf("control column must admit wrapping within its allocated width: %s", body)
		}
		if !strings.Contains(body, content) || !strings.Contains(settingsText(columns[1]), label) || !strings.Contains(settingsText(columns[1]), message) {
			t.Fatal("layout rewrote controls or lost complete label/message text")
		}
		if len(settingsNodes(columns[1], "input")) != 3 || len(settingsNodes(columns[1], "script")) != 0 {
			t.Fatal("layout changed native controls or added runtime behavior")
		}
	}
}
