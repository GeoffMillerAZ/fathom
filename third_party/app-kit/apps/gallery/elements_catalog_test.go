package gallery

import (
	"encoding/json"
	"html"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

func TestElementsTranslatorGalleryNeedsNoLicensedStore(t *testing.T) {
	h := New()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	res, body := get(t, h, "/api/catalog/search?q=Elements+conversion&profile=cli")
	var page catalog.Page
	if res.StatusCode != 200 || json.Unmarshal([]byte(body), &page) != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != "reference.elements-translator" || page.Revision != c.Revision || len(body) > 4<<10 {
		t.Fatal("Elements metadata discovery mismatch")
	}
	for _, forbidden := range []string{`"source"`, `"usage"`, "TranslateElements", "<el-disclosure"} {
		if strings.Contains(body, forbidden) {
			t.Fatal("summary leaked exact source/contract", forbidden)
		}
	}
	res, body = get(t, h, "/api/catalog/reference.elements-translator")
	var got struct {
		Revision string        `json:"revision"`
		Entry    catalog.Entry `json:"entry"`
	}
	want, ok := c.Get("reference.elements-translator")
	if res.StatusCode != 200 || json.Unmarshal([]byte(body), &got) != nil || !ok || got.Revision != c.Revision || !reflect.DeepEqual(got.Entry, want) {
		t.Fatal("Elements exact record diverges")
	}
	res, body = get(t, h, "/catalog/reference.elements-translator?profile=cli&theme=dark")
	if res.StatusCode != 200 || strings.Count(body, "<main ") != 1 {
		t.Fatal("Elements detail route failed")
	}
	for _, text := range []string{"experimental", "not-verified", "No -save", "1.0.22", "Your application owns"} {
		if !strings.Contains(html.UnescapeString(body), text) {
			t.Fatal("Elements detail lost boundary", text)
		}
	}
	for _, profile := range []string{"go-web", "static-web", "react-web"} {
		res, body = get(t, h, "/api/catalog/search?q=Elements+conversion&profile="+profile)
		if res.StatusCode != 200 || json.Unmarshal([]byte(body), &page) != nil || page.Total != 0 || len(page.Items) != 0 {
			t.Fatal("CLI syntax was promoted to web compatibility", profile)
		}
	}
}
