package catalog

import (
	"slices"
	"strings"
	"testing"
)

func TestSettingsSectionsDiscoveryKeepsPresentationExperimental(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get("shell.settings-sections")
	if !ok || e.Kind != "recipe" || e.Maturity != "experimental" || e.Preview != "/shell/settings" || len(e.Consumers) != 0 || !slices.Equal(e.Profiles, []string{"go-web"}) {
		t.Fatal("settings layout fit or promotion claim changed")
	}
	if e.Source.Path != "appkit/ui/form/settings.go" || !slices.Equal(e.Source.Symbols, []string{"SettingsSection", "RenderSettingsSections"}) {
		t.Fatalf("settings source = %#v", e.Source)
	}
	for _, phrase := range []string{"before writing a response", "h3", "document-unique IDs", "reviewed HTML", "labeled form.RenderField", "Scan appkit/ui/form/*.go", "saving and recovery stay app-owned"} {
		if !strings.Contains(e.Usage, phrase) {
			t.Errorf("missing boundary %q", phrase)
		}
	}
	if !strings.Contains(strings.Join(e.Gaps, " "), "two consumer integrations") {
		t.Fatal("permanent promotion gate missing")
	}
}

func TestShellWorkflowDiscoveryDoesNotPromoteAnAccountBackend(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get("shell.account-workflow")
	if !ok || e.Kind != "recipe" || e.Maturity != "experimental" || e.Preview != "/shell" || len(e.Consumers) != 0 || !slices.Equal(e.Profiles, []string{"go-web"}) {
		t.Fatal("synthetic workflow fit or promotion claim changed")
	}
	if e.Source.Path != "appkit/ui/dashboard/account.go" || !slices.Equal(e.Source.Symbols, []string{"AccountDestinations", "Notice", "RenderNotice"}) {
		t.Fatalf("account recipe source = %#v", e.Source)
	}
	for _, phrase := range []string{
		"dashboard.AccountDestinations", "Group", "dashboard.RenderNav", "dashboard.RenderNotice",
		"scan appkit/ui/dashboard/*.go", "raw model fields are not a sanitizer",
		"controllers remain Gallery-only", "experimental composition", "not an account-backend API", "permission controls",
	} {
		if !strings.Contains(e.Usage, phrase) {
			t.Errorf("missing boundary %q", phrase)
		}
	}
	for _, phrase := range []string{"production auth", "save/recovery controller", "Browser keyboard", "Two consumer integrations"} {
		if !strings.Contains(strings.Join(e.Gaps, " "), phrase) {
			t.Errorf("missing proof gap %q", phrase)
		}
	}
	p, err := c.Search(Query{Text: "profile settings", Kind: "recipe", Profile: "go-web"})
	if err != nil || len(p.Items) == 0 || p.Items[0].ID != e.ID {
		t.Fatal("profile/settings task did not discover its implemented recipe", err)
	}
	p, err = c.Search(Query{Text: "profile settings", Kind: "recipe", Profile: "react-web"})
	if err != nil || p.Total != 0 {
		t.Fatal("Go demo recommended as a React integration", err)
	}
	shell, _ := c.Get("shell.layout")
	if shell.Kind != "primitive" || shell.Maturity != "available" || shell.Preview != "/shell" {
		t.Fatal("existing shell primitive identity was replaced or overpromoted")
	}
}
