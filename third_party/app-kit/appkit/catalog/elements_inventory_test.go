package catalog

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestElementsTranslatorDiscoveryDoesNotAdvertiseConvertedComponents(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get("reference.elements-translator")
	if !ok || e.Kind != "reference" || e.Maturity != "experimental" || !slices.Equal(e.Profiles, []string{"cli"}) || len(e.Layers) != 0 || len(e.Consumers) != 0 || e.Preview != "" {
		t.Fatal("Elements metadata overstates target availability or adoption")
	}
	if e.Source.Path != "internal/designconvert/elements.go" || !slices.Equal(e.Source.Symbols, []string{"TranslateElements", "VerifyElementsReplay"}) {
		t.Fatal("wrong implementation anchor")
	}
	for _, text := range []string{"assets-translate-elements", "elements-syntax", "No -save", "syntax@", "not certified web-component"} {
		if !strings.Contains(e.Usage, text) {
			t.Fatal("missing usage boundary", text)
		}
	}
	if !strings.Contains(strings.Join(e.Dependencies, " "), "host-provided @tailwindplus/elements 1.0.22") || !strings.Contains(strings.Join(e.Gaps, " "), "not-verified") {
		t.Fatal("missing runtime/evidence boundary")
	}
	for _, query := range []Query{{Text: "Elements conversion"}, {Text: "Elements conversion", Profile: "cli"}} {
		p, err := c.Search(query)
		if err != nil || p.Total != 1 || len(p.Items) != 1 || p.Items[0].ID != e.ID || p.Revision != c.Revision {
			t.Fatal("Elements discovery mismatch", err)
		}
		body, err := json.Marshal(p)
		if err != nil || len(body) > 4<<10 {
			t.Fatal("unbounded discovery")
		}
		for _, secret := range []string{`"source"`, `"usage"`, "dataAssetsTranslateElements", "<el-disclosure"} {
			if strings.Contains(string(body), secret) {
				t.Fatal("summary leaked full contract/source", secret)
			}
		}
	}
	for _, query := range []Query{
		{Text: "Elements conversion", Profile: "go-web"}, {Text: "Elements conversion", Profile: "static-web"},
		{Text: "Elements conversion", Profile: "react-web"}, {Text: "Elements conversion", Maturity: "proven"},
		{Text: "Elements conversion", Maturity: "available"}, {Text: "Elements conversion", Kind: "recipe"},
	} {
		p, err := c.Search(query)
		if err != nil || p.Total != 0 || len(p.Items) != 0 {
			t.Fatal("unsupported Elements target promoted", query, err)
		}
	}
}
