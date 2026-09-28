package extractor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// TestDialectManifests (§2.2) loads every embedded manifest and holds the
// cross-dialect rules: ids match directories and the queue, target levels
// match the queue, and the registry TSExtractor is built from covers every
// tags dialect.
func TestDialectManifests(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	if len(ds) == 0 {
		t.Fatal("no dialects embedded")
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	var queue []struct {
		ID          string `json:"id"`
		TargetLevel string `json:"target_level"`
		Engine      string `json:"engine"`
	}
	if err := json.Unmarshal(raw, &queue); err != nil {
		t.Fatalf("queue: %v", err)
	}
	byID := map[string]int{}
	for i, q := range queue {
		byID[q.ID] = i
	}
	tags := map[string]bool{}
	for _, d := range ds {
		i, ok := byID[d.ID]
		if !ok {
			t.Errorf("dialect %s has no queue entry", d.ID)
			continue
		}
		if queue[i].TargetLevel != d.TargetLevel {
			t.Errorf("dialect %s: target_level %s, queue says %s", d.ID, d.TargetLevel, queue[i].TargetLevel)
		}
		if queue[i].Engine != d.Engine {
			t.Errorf("dialect %s: engine %s, queue says %s", d.ID, d.Engine, queue[i].Engine)
		}
		if d.Engine == "tags" {
			tags[d.ID] = true
		}
	}
	for _, id := range TagsDialects() {
		if !tags[id] {
			t.Errorf("TagsDialects lists %s, which is not a tags manifest", id)
		}
		delete(tags, id)
	}
	for id := range tags {
		t.Errorf("tags manifest %s missing from the registry", id)
	}
}

// TestLoadDialectsRefuses holds the cross-dialect rules against bad
// manifests: each case must fail with an error naming the rule.
func TestLoadDialectsRefuses(t *testing.T) {
	base := func(id, kind, ext string) string {
		return `{"id":"` + id + `","display":"X","kind":"language","engine":"tags","lang_tag":"x",
"match":{"extensions":["` + ext + `"]},"grammar":{"blob":"x","entry_point":"tree_sitter_x"},
"queries":{"tags":"tags.scm"},"kinds":{"f":{"source_kind":"` + kind + `","canonical":"function"}},
"required_kinds":["f"],"target_level":"L1","queue_id":"` + id + `"}`
	}
	fs := func(files map[string]string) fstest.MapFS {
		m := fstest.MapFS{}
		for p, c := range files {
			m[p] = &fstest.MapFile{Data: []byte(c)}
		}
		return m
	}
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"dir mismatch": {map[string]string{"dialects/a/dialect.json": base("b", "AF", ".a"), "dialects/a/tags.scm": ""}, "is not its directory"},
		"dup source kind": {map[string]string{
			"dialects/a/dialect.json": base("a", "Dup", ".a"), "dialects/a/tags.scm": "",
			"dialects/b/dialect.json": base("b", "Dup", ".b"), "dialects/b/tags.scm": "",
		}, "already emitted"},
		"dup extension": {map[string]string{
			"dialects/a/dialect.json": base("a", "AF", ".x"), "dialects/a/tags.scm": "",
			"dialects/b/dialect.json": base("b", "BF", ".x"), "dialects/b/tags.scm": "",
		}, "already claimed"},
		"bad canonical":       {map[string]string{"dialects/a/dialect.json": strings.Replace(base("a", "AF", ".a"), `"function"`, `"gizmo"`, 1), "dialects/a/tags.scm": ""}, "closed list"},
		"missing query":       {map[string]string{"dialects/a/dialect.json": base("a", "AF", ".a")}, "query tags.scm"},
		"unknown field":       {map[string]string{"dialects/a/dialect.json": strings.Replace(base("a", "AF", ".a"), `"display"`, `"level_met":"L4","display"`, 1), "dialects/a/tags.scm": ""}, "unknown field"},
		"bad sniff":           {map[string]string{"dialects/a/dialect.json": strings.Replace(base("a", "AF", ".a"), `"extensions":[".a"]`, `"extensions":[".a"],"sniff":{"extensions":[".q"],"pattern":"(","priority":0}`, 1), "dialects/a/tags.scm": ""}, "match.sniff.pattern"},
		"required undeclared": {map[string]string{"dialects/a/dialect.json": strings.Replace(base("a", "AF", ".a"), `"required_kinds":["f"]`, `"required_kinds":["class"]`, 1), "dialects/a/tags.scm": ""}, "not a declared kind"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadDialects(fs(c.files))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
}
