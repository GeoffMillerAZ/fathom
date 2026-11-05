package lang_test

import (
	"encoding/json"
	"fmt"
	"github.com/geoffmilleraz/fathom/pkg/lang"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A Helm template's if/else branches repeat a key, which yaml.v3 refuses; and a
// document that does not read says so. Design rule 8 names Helm as a reader
// that reports, and the reader used to skip such a document in silence.
func TestHelmTemplateReportsWhatItDoesNotRead(t *testing.T) {
	r := newReader(t)
	const chart = "chart/Chart.yaml"
	branches := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n" +
		"{{- if .Values.a }}\n  strategy:\n    type: Recreate\n{{- else }}\n  strategy:\n    type: RollingUpdate\n{{- end }}\n" +
		"  template:\n    spec:\n      containers:\n      - name: c\n        image: nginx:1\n"
	got := extract(t, r, "chart/templates/deploy.yaml", branches, chart)
	if got.Dialect != "helm" {
		t.Fatalf("dialect %q", got.Dialect)
	}
	names := map[string]bool{}
	for _, s := range got.Symbols {
		names[s.Name] = true
	}
	if !names["Deployment/web"] || !names["nginx:1"] || got.Incomplete {
		t.Errorf("a template with two branches: incomplete=%v notes=%v symbols=%v", got.Incomplete, got.Notes, names)
	}

	broken := "apiVersion: v1\nkind: Service\nmetadata:\n  name: ok\n---\nkind: [unclosed\nmetadata: {name:\n"
	got = extract(t, r, "chart/templates/svc.yaml", broken, chart)
	if !got.Incomplete || len(got.Notes) == 0 || !strings.HasPrefix(got.Notes[0], "incomplete: ") {
		t.Errorf("a broken second document reads incomplete=%v notes=%v", got.Incomplete, got.Notes)
	}
	var kept bool
	for _, s := range got.Symbols {
		kept = kept || s.Name == "Service/ok"
	}
	if !kept {
		t.Errorf("the first document is lost: %+v", got.Symbols)
	}
	if ok := extract(t, r, "chart/templates/svc.yaml", "apiVersion: v1\nkind: Service\nmetadata:\n  name: {{ .Release.Name }}\n", chart); ok.Incomplete {
		t.Errorf("a plain template reads incomplete: %v", ok.Notes)
	}
}

// A Helm template reads a mapping that repeats a key, or is wide, from the nodes
// as the Kubernetes reader does (maxDecodeMapKeys): 4,000 repeats of a key held
// 2.2 GB in yaml.v3's decode, and 100,000 distinct keys took 35 s.
func TestHelmTemplateRepeatedAndWideMappingsAreLinear(t *testing.T) {
	r := newReader(t)
	const head = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	within(t, 60*time.Second, "4,000 repeats", func() {
		extract(t, r, "chart/templates/d.yaml", head+strings.Repeat("z: 1\n", 4_000), "chart/Chart.yaml")
	})
	runtime.ReadMemStats(&after)
	if grew := (after.TotalAlloc - before.TotalAlloc) >> 20; grew > 256 {
		t.Errorf("4,000 repeats of a key allocated %d MiB", grew)
	}
	var b strings.Builder
	b.WriteString(head)
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(&b, "k%d: 1\n", i)
	}
	within(t, 20*time.Second, "100,000 keys", func() { extract(t, r, "chart/templates/d.yaml", b.String(), "chart/Chart.yaml") })
}

// A notebook cell the Python reader cannot read within its budget used to
// vanish: the same code as a .py read Incomplete with the limit named, and the
// notebook read complete with the cell's symbols missing.
func TestNotebookCellOverBudgetIsReported(t *testing.T) {
	r := newReader(t)
	nb := func(cells ...string) string {
		type cell struct {
			CellType string   `json:"cell_type"`
			Source   []string `json:"source"`
		}
		var cs []cell
		for _, c := range cells {
			cs = append(cs, cell{"code", strings.SplitAfter(c, "\n")})
		}
		b, err := json.MarshalIndent(map[string]any{
			"cells": cs, "nbformat": 4, "nbformat_minor": 5,
			"metadata": map[string]any{"kernelspec": map[string]any{"language": "python", "name": "python3"}},
		}, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	big := nb("def keep():\n    return 1\n", manyDefs(33_000))
	if len(big) > lang.DefaultMaxFileBytes {
		t.Fatalf("setup: %d bytes is over the file limit", len(big))
	}
	got := extract(t, r, "n/big.ipynb", big)
	if got.Skipped != "" || got.Dialect != "jupyter" {
		t.Fatalf("setup: skipped %q dialect %q", got.Skipped, got.Dialect)
	}
	if !got.Incomplete || len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "notebook cell 2") || strings.Contains(got.Notes[0], "\n") {
		t.Fatalf("incomplete=%v notes=%q", got.Incomplete, got.Notes)
	}
	var kept bool
	for _, s := range got.Symbols {
		kept = kept || s.Name == "keep"
	}
	if !kept {
		t.Errorf("the cell that read is lost: %+v", got.Symbols)
	}
	if ok := extract(t, r, "n/small.ipynb", nb("def keep():\n    return 1\n", manyDefs(200))); ok.Incomplete || len(ok.Notes) != 0 {
		t.Errorf("a small notebook reads incomplete: %v", ok.Notes)
	}
}
