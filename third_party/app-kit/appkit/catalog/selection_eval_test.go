package catalog_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/catalogtest"
)

func TestTaskSelectionEvaluation(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	catalogtest.Run(t, c, func(t *testing.T, q catalog.Query) []byte {
		t.Helper()
		page, err := c.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		return body
	})
}

func TestStaticContentDiscoveryMetadataRemainsBounded(t *testing.T) {
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.Get("content.docs")
	if !ok {
		t.Fatal("content.docs missing")
	}
	for _, task := range []string{"find fixed static content example", "find fixed FAQ example without JavaScript"} {
		if !slices.Contains(entry.Tasks, task) {
			t.Errorf("content.docs does not advertise supported task %q", task)
		}
	}
	for _, want := range []string{
		"separate experimental fixed Gallery exporter",
		"get_kit_decision(\"static-content-delivery\")",
		"docs/design/static-content-delivery.md",
		"go run ./apps/gallery/cmd/static-content",
		"CSS build prerequisite and fresh absolute output paths",
		"not a generic site generator, CMS, or certified static profile",
	} {
		if !strings.Contains(entry.Usage, want) {
			t.Errorf("content.docs usage missing %q", want)
		}
	}
	if !slices.Equal(entry.Profiles, []string{"go-web"}) || entry.Maturity != "available" {
		t.Fatalf("experimental exporter changed primitive availability: profiles=%v maturity=%q", entry.Profiles, entry.Maturity)
	}
}
