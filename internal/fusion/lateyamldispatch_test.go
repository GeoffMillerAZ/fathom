package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// lateTestDialect is an injected stand-in for a late dialect: a file with a
// top-level groups: list is a "prometheus" file.
const lateTestSpec = `{
  "decode": "yaml",
  "detect": {"content_sniff": "(?m)^groups[ \\t]*:"},
  "entities": [
    {"select": "$", "where": {"is_map": true}, "kind": "package", "source_kind": "PromFile", "name": "$file"},
    {"select": "$.groups[*]", "where": {"is_map": true}, "kind": "rule", "source_kind": "PromRuleGroup", "name": ".name"}
  ]
}`

func injectLateDialect(t *testing.T, id string) {
	t.Helper()
	spec, err := extractor.ParseDocSpec([]byte(lateTestSpec))
	if err != nil {
		t.Fatal(err)
	}
	x, err := extractor.NewDocExtractor(spec)
	if err != nil {
		t.Fatal(err)
	}
	old := lateYAMLLookup
	lateYAMLLookup = func(got string) *extractor.DocExtractor {
		if got == id {
			return x
		}
		return nil
	}
	t.Cleanup(func() { lateYAMLLookup = old })
}

const lateRules = "groups:\n  - name: node\n    rules:\n      - alert: Down\n        expr: up == 0\n"

func lateCorpus() map[string]string {
	return map[string]string{
		"alerts/rules.yml":             lateRules,
		"k8s/rules.yaml":               "apiVersion: monitoring.coreos.com/v1\nkind: PrometheusRule\nspec:\n  groups:\n    - name: x\n",
		".github/workflows/groups.yml": "groups:\n  - name: x\non: push\njobs: {}\n",
		"misc/notes.yml":               "title: hi\n",
	}
}

func TestLateYAML_NoDialectNoClaim(t *testing.T) {
	old := lateYAMLLookup
	lateYAMLLookup = func(string) *extractor.DocExtractor { return nil }
	t.Cleanup(func() { lateYAMLLookup = old })
	for rel, c := range lateCorpus() {
		if k := lateYAMLDetects(rel, []byte(c)); k != "" {
			t.Errorf("%s claimed as %q with no late dialect", rel, k)
		}
	}
	if k := Classify("alerts/rules.yml", []byte(lateRules), nil); k != ToolUnclassified {
		t.Errorf("Classify = %q, want unclassified", k)
	}
}

func TestLateYAML_InjectedClaimsAndRefuses(t *testing.T) {
	injectLateDialect(t, "prometheus")
	if k := lateYAMLDetects("alerts/rules.yml", []byte(lateRules)); k != ToolPrometheus {
		t.Errorf("groups file = %q, want prometheus", k)
	}
	if k := Classify("alerts/rules.yml", []byte(lateRules), nil); k != ToolPrometheus {
		t.Errorf("Classify groups file = %q, want prometheus", k)
	}
	for _, rel := range []string{"k8s/rules.yaml", ".github/workflows/groups.yml", "misc/notes.yml"} {
		if k := lateYAMLDetects(rel, []byte(lateCorpus()[rel])); k != "" {
			t.Errorf("%s claimed as %q, want refused", rel, k)
		}
	}
	// A claim needs a decodable root: sniff alone is not enough.
	if k := lateYAMLDetects("a.yml", []byte("groups: [unterminated\n")); k != "" {
		t.Errorf("undecodable file claimed as %q", k)
	}
	// An earlier signal still wins (Compose).
	if k := Classify("c.yml", []byte("services:\n  web:\n    image: x\ngroups:\n  - name: a\n"), nil); k != ToolCompose {
		t.Errorf("earlier verdict changed: %q", k)
	}
}

func lateIngest(t *testing.T) (*Pipeline, store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	writeCorpus(t, dir, lateCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "late", Path: dir}); err != nil {
		t.Fatal(err)
	}
	return p, st, dir
}

func TestIngest_LateYAMLReach(t *testing.T) {
	injectLateDialect(t, "prometheus")
	_, st, _ := lateIngest(t)
	got := crSourceKinds(t, st, "late")
	if !crHasKind(got["alerts/rules.yml"], "PromRuleGroup") {
		t.Errorf("alerts/rules.yml read as %q, want PromRuleGroup", got["alerts/rules.yml"])
	}
	for _, rel := range []string{"k8s/rules.yaml", ".github/workflows/groups.yml", "misc/notes.yml"} {
		if crHasKind(got[rel], "PromRuleGroup") {
			t.Errorf("%s read as a late dialect: %q", rel, got[rel])
		}
	}
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	if d := p.ContentDialect("alerts/rules.yml", []byte(lateRules), nil); d != "prometheus" {
		t.Errorf("ContentDialect = %q, want prometheus", d)
	}
}

func TestReingestFile_LateYAMLParity(t *testing.T) {
	injectLateDialect(t, "prometheus")
	p, st, dir := lateIngest(t)
	before := crSourceKinds(t, st, "late")
	for rel := range lateCorpus() {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("# edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(context.Background(), st, "late", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	after := crSourceKinds(t, st, "late")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}
