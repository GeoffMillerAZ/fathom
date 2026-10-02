package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Helm's values tree names the keys: one path per key down valuesMaxDepth,
// descending only into non-empty maps (the dialect's rule, mirrored).
func TestValueKeyPaths(t *testing.T) {
	vals := map[string]any{
		"image":   map[string]any{"repository": "example.com/app", "tag": nil},
		"empty":   map[string]any{},
		"list":    []any{map[string]any{"a": 1}},
		"enabled": false,
	}
	got := valueKeyPaths(vals, valuesMaxDepth)
	sort.Strings(got)
	want := []string{"empty", "enabled", "image", "image.repository", "image.tag", "list"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("valueKeyPaths = %v, want %v", got, want)
	}
}

// Ablation renames exactly one define's name literal, so Helm no longer
// resolves that name and nothing else in the file moves.
func TestAblateDefine(t *testing.T) {
	src := "{{- define \"app.name\" -}}x{{- end }}\n{{- define \"app.fullname\" -}}{{ include \"app.name\" . }}{{- end }}\n"
	got, ok := ablateDefine(src, "app.name", "ox-ablated")
	if !ok {
		t.Fatal("define not found")
	}
	want := "{{- define \"ox-ablated\" -}}x{{- end }}\n{{- define \"app.fullname\" -}}{{ include \"app.name\" . }}{{- end }}\n"
	if got != want {
		t.Fatalf("ablateDefine:\n got %q\nwant %q", got, want)
	}
	if _, ok := ablateDefine(src, "absent", "x"); ok {
		t.Fatal("an absent define must not be found")
	}
}

// A define is visible to a call in the same chart, a subchart of it or a
// parent of it: Helm renders a chart and its subcharts as one template set.
func TestVisible(t *testing.T) {
	for _, c := range []struct {
		callChart, defChart string
		want                bool
	}{
		{"a", "a", true},
		{"a/charts/b", "a", true},
		{"a", "a/charts/b", true},
		{"a", "c", false},
		{"ab", "a", false},
	} {
		if got := visible(c.callChart, c.defChart); got != c.want {
			t.Errorf("visible(%q, %q) = %v, want %v", c.callChart, c.defChart, got, c.want)
		}
	}
}

// The tool decides: a chart helm renders offline contributes Helm's chart,
// values keys and every template entity of the files Helm loads; each define
// is attested by ablation (renamed, a probe include fails naming it), so an
// unused define is attested too, and a call to a name no define of the chart
// carries is unevidenced. A chart with a missing dependency contributes
// nothing.
func TestTierAChart(t *testing.T) {
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
	write("app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n")
	write("app/values.yaml", "image:\n  repository: example.com/app\nreplicas: 1\n")
	write("app/templates/_helpers.tpl", "{{- define \"app.name\" -}}app{{- end }}\n{{- define \"app.unused\" -}}u{{- end }}\n{{- define \"app.full\" -}}{{ include \"app.name\" . }}{{- end }}\n")
	write("app/templates/cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ include \"app.full\" . }}\ndata:\n  r: {{ .Values.replicas | quote }}\n{{- if false }}\n  x: {{ include \"app.dangling\" . }}\n{{- end }}\n")
	write("dep/Chart.yaml", "apiVersion: v2\nname: dep\nversion: 0.1.0\ndependencies:\n  - name: common\n    version: 1.x.x\n    repository: oci://registry.example.com/charts\n")
	write("dep/templates/cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: dep\n")
	scratch := t.TempDir()
	res, err := tierA(dir, bin, scratch, 2)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range res.defs {
		names = append(names, d.Path+":"+d.Kind+":"+d.Name)
	}
	sort.Strings(names)
	want := []string{
		"app/Chart.yaml:package:app",
		"app/templates/_helpers.tpl:macro:app.full",
		"app/templates/_helpers.tpl:macro:app.name",
		"app/templates/_helpers.tpl:macro:app.unused",
		"app/templates/_helpers.tpl:macro_ref:app.name",
		"app/templates/cm.yaml:key:replicas",
		"app/templates/cm.yaml:macro_ref:app.full",
		"app/values.yaml:key:image",
		"app/values.yaml:key:image.repository",
		"app/values.yaml:key:replicas",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("tierA definitions:\n got %s\nwant %s", strings.Join(names, "\n     "), strings.Join(want, "\n     "))
	}
	if len(res.refs) != 1 || res.refs[0] != (ref{"app/templates/_helpers.tpl", "app.full", "app.name"}) {
		t.Fatalf("tierA references: %+v", res.refs)
	}
	if res.st.attested != 3 || res.st.unevidencedCalls != 1 || res.st.outOfScope["missing-dependency-oci"] != 1 {
		t.Fatalf("tierA stats: %+v", res.st)
	}
	if !reflect.DeepEqual(res.files, []string{"app/Chart.yaml", "app/templates/_helpers.tpl", "app/templates/cm.yaml", "app/values.yaml"}) {
		t.Fatalf("tierA files: %v", res.files)
	}
}

// A subchart that renders only inside its parent (it reads values the parent
// sets for it) is measured by the parent's render, its defines ablated in the
// parent's copy.
func TestTierASubchartInParent(t *testing.T) {
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
	write("p/Chart.yaml", "apiVersion: v2\nname: p\nversion: 0.1.0\n")
	write("p/values.yaml", "sub:\n  x:\n    z: 1\n")
	write("p/charts/sub/Chart.yaml", "apiVersion: v2\nname: sub\nversion: 0.1.0\n")
	write("p/charts/sub/templates/_h.tpl", "{{- define \"sub.y\" -}}{{ .Values.x.z }}{{- end }}\n")
	write("p/charts/sub/templates/cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: s\ndata:\n  y: {{ include \"sub.y\" . | quote }}\n")
	res, err := tierA(dir, bin, t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.st.rendered != 2 || res.st.attested != 1 || res.st.unevidencedCalls != 0 || res.st.keysUnlocated != 0 {
		t.Fatalf("stats: %+v", res.st)
	}
	if !slicesContains(res.files, "p/charts/sub/templates/cm.yaml") {
		t.Fatalf("files: %v", res.files)
	}
}

func slicesContains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
