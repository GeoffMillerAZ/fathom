package catalog

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestHeroAdapterExactMetadataAndBoundedDiscovery(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}

	e, ok := c.Get("reference.hero-adapter")
	if !ok {
		t.Fatal("missing hero adapter metadata")
	}
	if e.ID != "reference.hero-adapter" || e.Name != "Private split-hero adapter CLI" || e.Kind != "reference" || e.Maturity != "experimental" {
		t.Fatalf("hero identity or maturity differs: %+v", e)
	}
	if !slices.Equal(e.Profiles, []string{"cli"}) || len(e.Layers) != 0 || len(e.Consumers) != 0 || e.Preview != "" {
		t.Fatalf("hero metadata overstates renderer fit, adoption, or preview: %+v", e)
	}
	wantSource := Source{Path: "internal/designadapt/hero_delivery.go", Symbols: []string{"WriteHeroDelivery", "CheckHeroDelivery"}}
	if !reflect.DeepEqual(e.Source, wantSource) {
		t.Fatalf("hero source anchors differ\ngot:  %+v\nwant: %+v", e.Source, wantSource)
	}

	dependencies := strings.Join(e.Dependencies, " ")
	for _, phrase := range []string{
		"marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675",
		"html/sections/heroes/09-split-with-image.html",
		"source asset URLs are provenance only and are not bundled",
	} {
		if !strings.Contains(dependencies, phrase) {
			t.Errorf("hero dependency boundary omits %q", phrase)
		}
	}
	for _, phrase := range []string{
		`get_kit_decision("hero-adapter")`,
		"appkit data assets-adapt-hero -root PRIVATE_DIRECTORY",
		"-out NEW_PRIVATE_DIRECTORY",
		"-check PRIVATE_DIRECTORY",
		"six immutable files",
		"hero.templ, hero.go, hero.css, defaults.json, USAGE.md and artifact.json",
		"Compile a COPY in an app-owned private module with no app-kit runtime dependency",
		"all six deliberately blank URL fields",
		"constructor errors before response output",
		"NewHero preserves the original source composition",
		"NewHeroEnhanced explicitly opts into hero.css loaded after source CSS",
		"decorative images require empty alt",
		"never downloaded or bundled",
	} {
		if !strings.Contains(e.Usage, phrase) {
			t.Errorf("hero usage omits %q", phrase)
		}
	}
	for _, phrase := range []string{"all six routes and image URLs", "asset rights and delivery", "decorative-image and alt decision", "consumer task, browser, visual and accessibility acceptance"} {
		if !strings.Contains(strings.Join(e.AppOwns, " "), phrase) {
			t.Errorf("hero app ownership omits %q", phrase)
		}
	}
	if !slices.Equal(e.Proof, []string{"go test ./internal/designadapt", "go test ./cmd/appkit", "go test ./cmd/appkit-mcp"}) {
		t.Fatalf("hero proof commands differ: %q", e.Proof)
	}
	gaps := strings.Join(e.Gaps, " ")
	for _, phrase := range []string{"source identity", "compiler and DOM", "private replay", "browser and standalone consumer proof", "Consumer task", "certification remain unproven"} {
		if !strings.Contains(gaps, phrase) {
			t.Errorf("hero evidence boundary omits %q", phrase)
		}
	}
	wantProvenance := Provenance{
		Source:   "first-party",
		License:  "Metadata contains no licensed source bodies; generated derivatives retain original private-source restrictions",
		Audience: "metadata available without private archives; execution requires independently verified original access",
	}
	if !reflect.DeepEqual(e.Provenance, wantProvenance) {
		t.Fatalf("hero private provenance differs\ngot:  %+v\nwant: %+v", e.Provenance, wantProvenance)
	}

	for _, q := range []Query{{Text: "split hero private adapter"}, {Text: "split hero private adapter", Profile: "cli"}} {
		page, err := c.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != e.ID {
			t.Fatalf("hero adapter not exactly discoverable for %+v: %+v", q, page)
		}
	}
	for _, q := range []Query{
		{Text: "split hero private adapter", Profile: "go-web"},
		{Text: "split hero private adapter", Profile: "static-web"},
		{Text: "split hero private adapter", Profile: "react-web"},
		{Text: "split hero private adapter", Kind: "recipe"},
		{Text: "split hero private adapter", Maturity: "proven"},
		{Text: "split hero private adapter", Layer: "L4"},
	} {
		page, err := c.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 0 || len(page.Items) != 0 || page.NextCursor != "" {
			t.Errorf("unsupported hero fit promoted for %+v: %+v", q, page)
		}
	}

	encoded, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"<", "class=", "https://", ".png", ".jpg", ".jpeg", ".webp", ".svg"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("hero metadata includes licensed body/class/image material marker %q", forbidden)
		}
	}
}
