package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRemoteEntries(t *testing.T) {
	for v, want := range map[string]bool{
		"https://example.com/raw/x.yaml":       true,
		"github.com/kubernetes-sigs/kustomize": true,
		"git@github.com:org/repo.git//dir":     true,
		"git::https://example.com/repo.git":    true,
		"../base":                              false,
		"./service.yaml":                       false,
		"overlays/prod":                        false,
		"gitlab-config.yaml":                   false,
	} {
		if got := remoteRE.MatchString(v); got != want {
			t.Errorf("remote(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestTargetFile(t *testing.T) {
	work := t.TempDir()
	for _, f := range []string{"base/kustomization.yaml", "base/pod.yaml", "yml/kustomization.yml", "bare/pod.yaml", "overlay/kustomization.yaml"} {
		p := filepath.Join(work, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x: 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ks := map[string]bool{"base/kustomization.yaml": true, "yml/kustomization.yml": true, "overlay/kustomization.yaml": true}
	for _, c := range []struct {
		dir, value, want string
		ok               bool
	}{
		{"overlay", "../base", "base/kustomization.yaml", true},
		{"overlay", "../base/pod.yaml", "base/pod.yaml", true},
		{"overlay", "../yml", "yml/kustomization.yml", true},
		{"overlay", "../bare", "", false},       // a directory with no kustomization
		{"overlay", "../../outside", "", false}, // climbs out of the subject
		{"overlay", "../missing", "", false},
		{"", ".", "", false}, // the subject's own root
	} {
		got, ok := targetFile(work, c.dir, c.value, ks)
		if got != c.want || ok != c.ok {
			t.Errorf("targetFile(%q, %q) = %q, %v; want %q, %v", c.dir, c.value, got, ok, c.want, c.ok)
		}
	}
}

// TestDecode reads a kustomization with yq when it is installed: entries at their
// original index and line, a scalar in a map-list and a non-string skipped.
func TestDecode(t *testing.T) {
	yq, err := exec.LookPath("yq")
	if err != nil {
		t.Skip("yq not installed")
	}
	src := "# c\nresources:\n  - a.yaml\n  - 7\n  - b.yaml\npatches:\n  - path: p.yaml\n  - just-a-string\nimages:\n  - name: web\n    newTag: v1\nconfigMapGenerator:\n  - name: cfg\n"
	r := runner{yq: yq, timeout: 30 * time.Second, env: os.Environ()}
	kf, err := decode(context.Background(), r, yqProgram(), "x/kustomization.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Field string
		I     int
		V     string
		L     int
	}
	var got []row
	for _, e := range kf.entries {
		got = append(got, row{e.Field, e.Index, e.Value, e.Line})
	}
	want := []row{
		{"resources", 0, "a.yaml", 3}, {"resources", 2, "b.yaml", 5},
		{"patches", 0, "p.yaml", 7},
		{"images", 0, "web", 10},
		{"configMapGenerator", 0, "cfg", 13},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entries\n got: %v\nwant: %v", got, want)
	}
	if kf.line != 2 {
		t.Errorf("root line = %d, want 2 (the first key, after the comment)", kf.line)
	}
	// yq drops the lines that open a file (comments, blanks, a document marker) from its
	// numbering; the oracle adds them back
	lead := "# c1\n# c2\n\n---\nresources:\n  - a.yaml\n"
	kf, err = decode(context.Background(), r, yqProgram(), "x/kustomization.yaml", []byte(lead))
	if err != nil {
		t.Fatal(err)
	}
	if kf.line != 5 || len(kf.entries) != 1 || kf.entries[0].Line != 6 {
		t.Errorf("after a 4-line preamble: root line %d (want 5), entries %v (want one at line 6)", kf.line, kf.entries)
	}
}
