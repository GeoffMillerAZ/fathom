package catalog

import (
	"strings"
	"testing"
)

func TestSpreadsheetExportDiscoveryKeepsBoundaries(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	page, err := c.Search(Query{Text: "spreadsheet table export", Profile: "go-web"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "analytics.comparison" {
		t.Fatalf("wrong selection: %+v", page.Items)
	}
	entry, ok := c.Get(page.Items[0].ID)
	if !ok {
		t.Fatal("missing exact capability")
	}
	for _, text := range []string{"analytics.WriteTableCSV", "empty selections contain only headers", "WriteCSV for empty-selection provenance", "/analytics/recipe/table.csv", "old downloads stay unchanged"} {
		if !strings.Contains(entry.Usage, text) {
			t.Errorf("missing export contract %q", text)
		}
	}
	if entry.Maturity != "experimental" {
		t.Fatal("export promoted whole recipe")
	}
}
