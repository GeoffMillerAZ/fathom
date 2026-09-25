package gallery

import (
	"encoding/json"
	"html"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

func TestPrivateAdapterCatalogRoutesNeedNoLicensedStore(t *testing.T) {
	h := New() // No private source options or store access.
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	res, body := get(t, h, "/api/catalog/search?q=private+adapter&profile=cli&kind=reference")
	var page catalog.Page
	if res.StatusCode != 200 || json.Unmarshal([]byte(body), &page) != nil || page.Total != 9 || len(page.Items) != 9 || page.Revision != c.Revision {
		t.Fatal("shared private tooling metadata not discoverable without an archive store")
	}
	if len(body) > 8<<10 {
		t.Fatalf("private adapter summary exceeds bounded discovery budget: %d bytes", len(body))
	}
	for _, forbidden := range []string{`"source"`, `"usage"`, `"proof"`, `"provenance"`, "WritePricingDelivery", "WriteDirectoryDelivery", "WriteFormDelivery", "WriteNavigationDelivery", "sourceExpected", "sourceDefaults", "<option"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("bounded Gallery discovery exposed contract/source material %q", forbidden)
		}
	}
	adapters := []struct {
		id, query string
	}{
		{"reference.pricing-adapter", "pricing private adapter"},
		{"reference.directory-adapter", "directory table private adapter"},
		{"reference.form-adapter", "form email field private adapter"},
		{"reference.navigation-adapter", "page navigation private adapter"},
		{"reference.faq-adapter", "FAQ help private adapter"},
		{"reference.hero-adapter", "hero private adapter"},
		{"reference.menu-adapter", "dropdown menu private adapter"},
		{"reference.drawer-adapter", "profile drawer private adapter"},
		{"reference.navbar-adapter", "responsive navbar private adapter"},
	}
	if len(adapters) != 9 {
		t.Fatalf("private adapter Gallery inventory count = %d, want 9", len(adapters))
	}
	for _, adapter := range adapters {
		id := adapter.id
		res, body = get(t, h, "/api/catalog/search?q="+strings.ReplaceAll(adapter.query, " ", "+")+"&profile=cli")
		var exactPage catalog.Page
		if res.StatusCode != 200 || json.Unmarshal([]byte(body), &exactPage) != nil || exactPage.Total != 1 || len(exactPage.Items) != 1 || exactPage.Items[0].ID != id || exactPage.Revision != c.Revision || len(body) > 4<<10 {
			t.Fatalf("bounded query for %s diverges from shared metadata", id)
		}
		for _, forbidden := range []string{`"source"`, `"usage"`, `"proof"`, `"provenance"`, "WriteNavigationDelivery", "sourceExpected", "sourceDefaults", "<option"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s query exposed contract/source material %q", id, forbidden)
			}
		}

		res, body := get(t, h, "/api/catalog/"+id)
		var got struct {
			Revision string        `json:"revision"`
			Entry    catalog.Entry `json:"entry"`
		}
		want, ok := c.Get(id)
		if res.StatusCode != 200 || json.Unmarshal([]byte(body), &got) != nil || !ok || got.Revision != c.Revision || !reflect.DeepEqual(got.Entry, want) {
			t.Fatalf("exact %s route diverges from shared metadata", id)
		}
		res, body = get(t, h, "/catalog/"+id+"?profile=cli&theme=dark")
		if res.StatusCode != 200 || strings.Count(body, "<main ") != 1 || strings.Contains(body, "#ZgotmplZ") {
			t.Fatal("adapter detail did not render as a normal catalog record")
		}
		visible := html.UnescapeString(body)
		for _, want := range []string{"experimental", "-out NEW_PRIVATE_DIRECTORY", "-check PRIVATE_DIRECTORY", "not-verified", "private-source restrictions", "Your application owns"} {
			if !strings.Contains(visible, want) {
				t.Errorf("%s detail omitted %q", id, want)
			}
		}
	}
	for _, profile := range []string{"go-web", "static-web", "react-web"} {
		res, body := get(t, h, "/api/catalog/search?q=private+adapter&profile="+profile)
		var page catalog.Page
		if res.StatusCode != 200 || json.Unmarshal([]byte(body), &page) != nil || page.Total != 0 || len(page.Items) != 0 {
			t.Fatalf("CLI adapter metadata falsely advertises %s component fit", profile)
		}
	}
}
