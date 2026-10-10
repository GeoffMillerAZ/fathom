package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestOwnedSourceRefsNamesAndContent (LB-Q-prometheus): a doc-engine dialect
// that claims names AND that ingest's dispatch claims by content (the
// late-YAML hub) is measured on both: prometheus.yml by its name, a rule file
// named alerts.yaml by its content, never a neighbour's file.
func TestOwnedSourceRefsNamesAndContent(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) core.SourceRef {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return core.SourceRef{Path: rel}
	}
	refs := []core.SourceRef{
		write("prometheus.yml", "scrape_configs:\n  - job_name: a\n"),
		write("alerts/alerts.yaml", "groups:\n  - name: g\n    rules:\n      - alert: Down\n        expr: up == 0\n"),
		write("k8s.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"),
		write("docker-compose.yml", "services:\n  web:\n    image: nginx\n"),
		write("notes.yml", "title: groups\n"),
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	content := func(rel string, b []byte) string { return pipe.ContentDialect(rel, b, refs) }
	prom := langbench.Manifest{ID: "prometheus", Engine: "doc", Match: langbench.ManifestMatch{Basenames: []string{"prometheus.yml"}}}
	var got []string
	for _, r := range ownedSourceRefs(prom, root, refs, nil, content) {
		got = append(got, r.Path)
	}
	if strings.Join(got, ",") != "prometheus.yml,alerts/alerts.yaml" {
		t.Errorf("prometheus owns %v, want [prometheus.yml alerts/alerts.yaml]", got)
	}
}
