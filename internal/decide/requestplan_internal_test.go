package decide

import (
	"slices"
	"testing"
)

// TestEverySiteDeclaresItsRequestFields holds I61's declarations to the
// code's site list: each site with a call site (AllSites) declares its unit
// and the request fields it sends, and nothing else is declared, so a new
// site cannot ask until it says what it sends and a declaration cannot
// outlive its site.
func TestEverySiteDeclaresItsRequestFields(t *testing.T) {
	var declared []string
	for _, d := range siteDeclarations {
		declared = append(declared, d.Site)
		if d.Unit == "" || len(d.Fields) == 0 {
			t.Errorf("site %s declares unit %q and fields %v, want both", d.Site, d.Unit, d.Fields)
		}
		for _, f := range d.Fields {
			if !slices.Contains(requestFields, f) {
				t.Errorf("site %s declares field %q, not one of %v", d.Site, f, requestFields)
			}
		}
		if slices.Contains(d.Fields, fieldImages) {
			t.Errorf("site %s declares images: no site sends one today", d.Site)
		}
	}
	got, want := slices.Sorted(slices.Values(declared)), slices.Sorted(slices.Values(AllSites))
	if !slices.Equal(got, want) {
		t.Fatalf("declared sites %v, want exactly AllSites %v", got, want)
	}
	if _, ok := siteDeclarationOf("ask"); ok {
		t.Fatalf("an opt-in site with no call site has a declaration")
	}
	unscoped, ok := siteDeclarationOf("")
	if !ok || slices.Contains(unscoped.Fields, fieldImages) {
		t.Fatalf("the unscoped declaration = %+v, %v: want the union of every site's fields", unscoped, ok)
	}
	for _, d := range siteDeclarations {
		for _, f := range d.Fields {
			if !slices.Contains(unscoped.Fields, f) {
				t.Errorf("the unscoped declaration lacks %s's field %s", d.Site, f)
			}
		}
	}
}
