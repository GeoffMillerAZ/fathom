package main

import "testing"

// TestSCSSFileEdgesResolveUnderTheScssDialect (LB-F466): the scss dialect's
// imports are resolved by its own manifest's strategy, so an oracle file's
// imports grade import_resolution and the file edges.
func TestSCSSFileEdgesResolveUnderTheScssDialect(t *testing.T) {
	if lang, ok := fileResolveLang["scss"]; !ok || lang != "scss" {
		t.Fatalf("fileResolveLang[scss] = %q, %v; want scss", lang, ok)
	}
	if lang, ok := importLang("scss"); !ok || lang != "scss" {
		t.Fatalf("importLang(scss) = %q, %v; want scss", lang, ok)
	}
}
