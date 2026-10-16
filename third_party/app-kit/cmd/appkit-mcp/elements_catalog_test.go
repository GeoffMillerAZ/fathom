package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
)

func TestElementsTranslatorMCPUsesSharedMetadataWithoutLicensedCorpora(t *testing.T) {
	ctx, session := catalogSession(t)
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"cli", "go-web", "static-web", "react-web"} {
		body := catalogCall(t, ctx, session, "search_capabilities", capabilitySearchInput{Query: "Elements conversion", Profile: profile})
		var got catalog.Page
		if json.Unmarshal(body, &got) != nil {
			t.Fatal("invalid discovery JSON")
		}
		want, err := c.Search(catalog.Query{Text: "Elements conversion", Profile: profile})
		if err != nil || !reflect.DeepEqual(got, want) || got.Revision != c.Revision || len(body) > 4<<10 {
			t.Fatal("Elements shared metadata/parity failed")
		}
		if profile == "cli" {
			if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != "reference.elements-translator" {
				t.Fatal("Elements CLI missing")
			}
		} else if got.Total != 0 || len(got.Items) != 0 {
			t.Fatal("Elements web target falsely promoted")
		}
		for _, forbidden := range []string{`"source"`, `"usage"`, "TranslateElements", "<el-disclosure"} {
			if strings.Contains(string(body), forbidden) {
				t.Fatal("Elements summary exposed source/contract")
			}
		}
	}
	body := catalogCall(t, ctx, session, "get_capability", capabilityGetInput{ID: "reference.elements-translator"})
	var got capabilityOutput
	want, ok := c.Get("reference.elements-translator")
	if json.Unmarshal(body, &got) != nil || !ok || got.Revision != c.Revision || !reflect.DeepEqual(got.Entry, want) {
		t.Fatal("Elements exact record diverges")
	}
}
