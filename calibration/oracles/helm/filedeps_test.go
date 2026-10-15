package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// helm dependency list's table: only file:// rows are kept, a two-word
// status stays whole, and a row with an empty repository column never shifts
// a kept row's columns.
func TestParseDepList(t *testing.T) {
	out := "NAME       \tVERSION\tREPOSITORY                     \tSTATUS       \n" +
		"common     \t2.x.x  \toci://registry-1.docker.io/x  \tmissing      \n" +
		"partials   \t0.1.0  \tfile://../partials             \tok           \n" +
		"crds       \t0.1.0  \tfile://./charts/crds           \twrong version\n" +
		"vendored   \t1.0.0  \t                               \tunpacked     \n\n"
	got := parseDepList(out)
	want := []depRow{
		{"partials", "0.1.0", "file://../partials", "ok"},
		{"crds", "0.1.0", "file://./charts/crds", "wrong version"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDepList = %+v, want %+v", got, want)
	}
}

// Helm's GetLocalPath joins the path after file:// to the chart directory;
// a path that leaves the subject or is absolute names no file of it.
func TestLocalPath(t *testing.T) {
	for _, c := range []struct{ dir, repo, want string }{
		{"charts/linkerd-control-plane", "file://../partials", "charts/partials"},
		{"charts/datahub", "file://./subcharts/datahub-gms", "charts/datahub/subcharts/datahub-gms"},
		{"a", "file://charts/b", "a/charts/b"},
		{"a", "file://../../x", ""},
		{"a", "file:///abs/x", ""},
	} {
		if got := localPath(c.dir, c.repo); got != c.want {
			t.Errorf("localPath(%q, %q) = %q, want %q", c.dir, c.repo, got, c.want)
		}
	}
}

// Against helm itself: an unpacked file:// subchart is attested where it
// lies; one beside the chart is attested after helm dependency update in a
// scratch copy (the subject is never written); one in a chart that also
// names an oci:// dependency is counted, not built; a path naming a chart of
// another name is no edge.
func TestFileDepsHelm(t *testing.T) {
	bin, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm not on PATH")
	}
	dir := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("lib/Chart.yaml", "apiVersion: v2\nname: lib\ntype: library\nversion: 0.1.0\n")
	write("app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.0.0\ndependencies:\n  - name: lib\n    version: 0.1.0\n    repository: file://../lib\n  - name: sub\n    version: 0.2.0\n    repository: file://./charts/sub\n")
	write("app/charts/sub/Chart.yaml", "apiVersion: v2\nname: sub\nversion: 0.2.0\n")
	write("mixed/Chart.yaml", "apiVersion: v2\nname: mixed\nversion: 1.0.0\ndependencies:\n  - name: lib\n    version: 0.1.0\n    repository: file://../lib\n  - name: common\n    version: 2.x.x\n    repository: oci://registry.invalid/charts\n")
	write("wrong/Chart.yaml", "apiVersion: v2\nname: wrong\nversion: 1.0.0\ndependencies:\n  - name: other\n    version: 0.1.0\n    repository: file://../lib\n")
	var charts []chart
	for _, rel := range []string{"lib/Chart.yaml", "app/Chart.yaml", "app/charts/sub/Chart.yaml", "mixed/Chart.yaml", "wrong/Chart.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		var o out
		c, ok := walkChart(&o, rel, b)
		if !ok {
			t.Fatalf("walkChart %s", rel)
		}
		charts = append(charts, c)
	}
	xs, st, err := fileDeps(dir, bin, t.TempDir(), charts)
	if err != nil {
		t.Fatal(err)
	}
	want := []xref{
		{"app/Chart.yaml", "app", "app/charts/sub/Chart.yaml", "sub"},
		{"app/Chart.yaml", "app", "lib/Chart.yaml", "lib"},
	}
	if !reflect.DeepEqual(xs, want) {
		t.Fatalf("fileDeps = %+v, want %+v", xs, want)
	}
	if st.deps != 4 || st.unpacked != 1 || st.built != 1 || st.notGraded["mixed-remote-not-built"] != 1 || len(st.notGraded) != 2 {
		t.Fatalf("stats %+v", st)
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "charts", "lib-0.1.0.tgz")); err == nil {
		t.Fatal("the subject was written: helm dependency update must run in the scratch copy")
	}
	for k := range st.notGraded {
		if k != "mixed-remote-not-built" && !strings.HasPrefix(k, "built-") && k != "update-failed" {
			t.Errorf("unexpected reason %q", k)
		}
	}
}

// The walk discloses the written ceiling only when an oci:// edge exists.
func TestOCICeiling(t *testing.T) {
	n := ociCeiling(174, 173, 1)
	if !strings.HasPrefix(n, "L3 ceiling: 173 of this subject's 174 dependency edges name oci:// registry charts") {
		t.Fatal(n)
	}
}
