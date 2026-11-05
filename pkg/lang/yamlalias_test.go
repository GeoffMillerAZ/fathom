package lang_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// An anchor that repeats a key holds 2.2 GB when yaml.v3 decodes it into a Go
// value, once per alias, wherever the alias sits: a list item, the whole list, a
// merge or a scalar field. A Chart.yaml guarded only at its root and its direct
// dependencies decoded it.
func TestChartAnchorsThatRepeatAKeyAreNotDecoded(t *testing.T) {
	r := newReader(t)
	const head = "apiVersion: v2\nname: x\nversion: 1.0.0\n"
	anchor := "x: &a\n" + strings.Repeat("  z: 1\n", 6_000)
	for name, src := range map[string]string{
		"list item":          head + anchor + "dependencies:\n- *a\n",
		"whole list":         head + anchor + "dependencies: *a\n",
		"merge":              head + anchor + "<<: *a\n",
		"scalar":             head + anchor + "description: *a\n",
		"a dependency field": head + anchor + "dependencies:\n- name: d\n  version: *a\n",
	} {
		t.Run(name, func(t *testing.T) {
			var got lang.Result
			within(t, 30*time.Second, name, func() { got = extract(t, r, "chart/Chart.yaml", src) })
			if !got.Incomplete || !hasReason(got.Notes) {
				t.Fatalf("read as complete: incomplete=%v notes=%v", got.Incomplete, got.Notes)
			}
		})
	}
	// A wide mapping nothing aliases is never decoded, so it does not refuse the
	// chart, and an anchor that is a plain list does.
	var b strings.Builder
	b.WriteString(head + "annotations:\n")
	for i := 0; i < 5_000; i++ {
		fmt.Fprintf(&b, "  k%d: v\n", i)
	}
	b.WriteString("dependencies:\n- name: d\n  version: 1.2.3\n  repository: https://charts.example.com\n")
	if got := extract(t, r, "chart/Chart.yaml", b.String()); got.Incomplete || len(got.Symbols) < 2 {
		t.Errorf("a wide annotations mapping loses the chart: incomplete=%v symbols=%d notes=%v", got.Incomplete, len(got.Symbols), got.Notes)
	}
	plain := head + "x: &d\n- name: d\n  version: 1.2.3\ndependencies: *d\n"
	if got := extract(t, r, "chart/Chart.yaml", plain); got.Incomplete || len(got.Symbols) < 2 {
		t.Errorf("an aliased dependency list is lost: incomplete=%v symbols=%d notes=%v", got.Incomplete, len(got.Symbols), got.Notes)
	}
}

// A document whose mappings the decode refuses (one wider than 1,000 keys, or one
// that repeats a key) is read from its nodes, and the nodes are read as YAML
// reads them: an alias is the node it names and a merge supplies the keys the
// mapping does not write. A Pod whose containers sit behind an alias lost its
// container, silently, the moment any mapping in the document was wide.
func TestNodeReaderFollowsAliasesAndMerges(t *testing.T) {
	r := newReader(t)
	var wide strings.Builder
	wide.WriteString("extra:\n")
	for i := 0; i < 1_001; i++ {
		fmt.Fprintf(&wide, "  k%d: 1\n", i)
	}
	const c = "c: &c\n- name: app\n  image: reg/a:1\n"
	const s = "s: &s\n  containers:\n  - name: app\n    image: reg/a:1\n"
	const pod = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: p\n"
	for name, doc := range map[string]string{
		"containers alias": pod + c + "spec:\n  containers: *c\n" + wide.String(),
		"spec alias":       pod + s + "spec: *s\n" + wide.String(),
		"spec merge":       pod + s + "spec:\n  <<: *s\n" + wide.String(),
		"merge sequence":   pod + s + "spec:\n  <<: [*s]\n" + wide.String(),
		"repeated key":     pod + c + "spec:\n  containers: *c\nx: 1\nx: 2\n",
		"plain":            pod + "spec:\n  containers:\n  - name: app\n    image: reg/a:1\n" + wide.String(),
	} {
		t.Run(name, func(t *testing.T) {
			got := extract(t, r, "k/pod.yaml", doc)
			var found bool
			for _, sym := range got.Symbols {
				found = found || (sym.Kind == "K8sContainer" && sym.Name == "reg/a:1")
			}
			if !found {
				t.Fatalf("the container is lost: incomplete=%v notes=%v symbols=%+v", got.Incomplete, got.Notes, got.Symbols)
			}
		})
	}
}

// A Chart.yaml field the struct reads that holds a mapping is walked by yaml.v3's
// decode before it fails with a type error: `name:` over 16,000 lines of `z: 1`
// (112 KB) held 2.2 GB. The values the struct reads cost what the document costs.
func TestChartFieldHoldingAMappingIsCheapToRefuse(t *testing.T) {
	r := newReader(t)
	body := strings.Repeat("  z: 1\n", 16_000)
	for name, src := range map[string]string{
		"name":                 "apiVersion: v2\nname:\n" + body + "version: 1.0.0\n",
		"version":              "apiVersion: v2\nname: x\nversion:\n" + body,
		"dependency name":      "apiVersion: v2\nname: x\nversion: 1.0.0\ndependencies:\n- name:\n" + strings.ReplaceAll(body, "  z", "    z"),
		"dependency version":   "apiVersion: v2\nname: x\nversion: 1.0.0\ndependencies:\n- name: d\n  version:\n" + strings.ReplaceAll(body, "  z", "    z"),
		"dependencies mapping": "apiVersion: v2\nname: x\nversion: 1.0.0\ndependencies:\n" + body,
	} {
		t.Run(name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			var got lang.Result
			within(t, 30*time.Second, name, func() { got = extract(t, r, "chart/Chart.yaml", src) })
			runtime.ReadMemStats(&after)
			if mb := (after.TotalAlloc - before.TotalAlloc) >> 20; mb > 200 {
				t.Errorf("allocated %d MB reading 16,000 lines", mb)
			}
			if !got.Incomplete || !hasReason(got.Notes) {
				t.Errorf("read as complete: incomplete=%v notes=%v", got.Incomplete, got.Notes)
			}
		})
	}
}
