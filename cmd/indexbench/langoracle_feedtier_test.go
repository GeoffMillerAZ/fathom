package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFeedTier (ruling C-R6, F5): a metric is judged at the weakest tier of
// the oracle files that feed it; a file that carries none of the metric's
// keys does not lower it, and a file naming no tier is tier A.
func TestFeedTier(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pre := write("pre.json", `{"tool":"clang-preprocessor","tier":"A","imports":[],"active_lines":{"a.cu":[[1,9]]}}`)
	calls := write("calls.json", `{"tool":"clang-ast","tier":"B","call_sites":[{"path":"a.cu","line":2,"col":3,"name":"f"}]}`)
	untiered := write("defs.json", `{"tool":"x","definitions":[]}`)
	for _, c := range []struct {
		name  string
		paths []string
		keys  []string
		want  string
	}{
		{"no feeding file", []string{untiered}, []string{"call_sites"}, "A"},
		{"preprocessor only", []string{pre, calls}, []string{"active_lines"}, "A"},
		{"calls lower it", []string{pre, calls}, []string{"call_sites", "active_lines"}, "B"},
		{"none given", nil, []string{"call_sites"}, "A"},
	} {
		got, err := feedTier(c.paths, c.keys...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: feedTier = %q, want %q", c.name, got, c.want)
		}
	}
	if got := weakerTier("A", "C"); got != "C" {
		t.Errorf("weakerTier(A, C) = %q, want C", got)
	}
	if got := weakerTier("B", "A"); got != "B" {
		t.Errorf("weakerTier(B, A) = %q, want B", got)
	}
}
