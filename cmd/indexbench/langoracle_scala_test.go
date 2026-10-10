package main

import "testing"

// TestFileResolveLangServesScala (LB-F438): the scala dialect names an import
// strategy now, so a reading without a SCIP index scores its file-resolved
// edges against a dump's imports (LB-F57), as kotlin's does.
func TestFileResolveLangServesScala(t *testing.T) {
	if lang, ok := fileResolveLang["scala"]; !ok || lang != "scala" {
		t.Fatalf("fileResolveLang[scala] = %q, %v; want scala", lang, ok)
	}
}
