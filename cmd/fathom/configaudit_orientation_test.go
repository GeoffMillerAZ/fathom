package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOrientationFixture(t *testing.T, root, pack string) {
	t.Helper()
	for path, body := range map[string]string{
		".fathom/pack.cue":     pack,
		"internal/verify/p.go": "package verify\n\nfunc ParsePlan() {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Every orientation anchor resolves, or the audit names it: a moved file or a
// renamed symbol is never dropped silently, and the run exits 1.
func TestOrientationAnchorsResolveOrAreNamed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	good := t.TempDir()
	writeOrientationFixture(t, good, `orientation: [
	{anchor: "internal/verify/p.go", why: "the parser"},
	{anchor: "internal/verify/p.go#ParsePlan", why: "the entry point"},
	{anchor: "internal/verify", why: "the package"},
]
`)
	var out, errB bytes.Buffer
	if code := cmdConfigAudit([]string{"--repo", good}, &out, &errB); code != 0 {
		t.Fatalf("resolving anchors must exit 0, got %d\n%s%s", code, out.String(), errB.String())
	}
	if !strings.Contains(out.String(), "3 anchor(s) resolve") {
		t.Errorf("a clean set is named, got %q", out.String())
	}

	bad := t.TempDir()
	writeOrientationFixture(t, bad, `orientation: [
	{anchor: "internal/verify/p.go#ParsePlan", why: "kept"},
	{anchor: "internal/gone/moved.go", why: "moved"},
	{anchor: "internal/verify/p.go#Renamed", why: "renamed"},
	{anchor: "../escape.go", why: "outside"},
]
`)
	out.Reset()
	errB.Reset()
	code := cmdConfigAudit([]string{"--repo", bad}, &out, &errB)
	if code != 1 {
		t.Fatalf("unresolved anchors must exit 1, got %d\n%s", code, out.String())
	}
	for _, want := range []string{"internal/gone/moved.go", "internal/verify/p.go#Renamed", "../escape.go", "3 of 4"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("audit must name %q, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "internal/verify/p.go#ParsePlan\n") {
		t.Errorf("a resolving anchor is not listed as unresolved:\n%s", out.String())
	}

	// A set that does not load is named, never dropped.
	broken := t.TempDir()
	writeOrientationFixture(t, broken, `orientation: [{anchor: "internal/verify/p.go"}]`+"\n")
	out.Reset()
	errB.Reset()
	if code := cmdConfigAudit([]string{"--repo", broken}, &out, &errB); code != 1 || !strings.Contains(out.String(), "orientation") {
		t.Fatalf("an invalid orientation set must be named with exit 1, got %d\n%s", code, out.String())
	}
}
