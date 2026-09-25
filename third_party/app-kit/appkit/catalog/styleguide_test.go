package catalog

import (
	"slices"
	"strings"
	"testing"
)

func TestLocalStyleguideDiscoveryDoesNotClaimConsumerPromotion(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get("recipe.styleguide.local")
	if !ok || e.Kind != "recipe" || e.Maturity != "experimental" || len(e.Consumers) != 0 || e.Preview != "" || !slices.Equal(e.Profiles, []string{"go-web"}) {
		t.Fatalf("local guide overstates availability or promotion: %+v", e)
	}
	for _, phrase := range []string{"-theme-source", "concrete candidate snapshot", "app-owned-styleguide"} {
		if !strings.Contains(e.Usage, phrase) {
			t.Fatal("missing guide boundary", phrase)
		}
	}
	page, err := c.Search(Query{Text: "theme editor", Profile: "go-web"})
	if err != nil || len(page.Items) == 0 || page.Items[0].ID != e.ID {
		t.Fatal("theme editor not discoverable", err)
	}
	for _, profile := range []string{"react-web", "static-web", "portable", "cli"} {
		p, err := c.Search(Query{Text: "theme editor", Profile: profile})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range p.Items {
			if item.ID == e.ID {
				t.Fatal("unsupported runtime recommended", profile)
			}
		}
	}
}
