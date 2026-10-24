package main

import "testing"

// Jinja2's extends, include and import statements resolve through the include-path
// strategy (an ancestor directory of the importing template is the loader root), so
// the file resolver serves it and a dump's imports are its file-edge truth
// (LB-F57, LB-Q-jinja2).
func TestFileResolveLangServesJinja2(t *testing.T) {
	if lang, ok := fileResolveLang["jinja2"]; !ok || lang != "jinja2" {
		t.Fatalf("fileResolveLang[jinja2] = %q, %v; want jinja2", lang, ok)
	}
}
