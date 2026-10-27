package mcp_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const entityFactsWorkload = `apiVersion: apps/v1
kind: Deployment
metadata: {name: api, namespace: shop}
spec:
  replicas: 2
  selector: {matchLabels: {app: api}}
  template:
    metadata: {labels: {app: api}}
    spec:
      containers:
        - name: api
          image: acme/api:1
          resources:
            limits:
              memory: 128Mi
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: api-canary, namespace: shop}
spec:
  replicas: 1
  selector: {matchLabels: {app: api-canary}}
  template:
    metadata: {labels: {app: api-canary}}
    spec:
      containers: [{name: api, image: "acme/api:2"}]
`

const entityFactsRules = `groups:
  - name: node
    rules:
      - alert: NodeDown
        expr: up{job="node"} == 0
        labels:
          severity: critical
          team: infra
      - alert: DiskFull
        expr: disk_free < 0.10
        labels:
          severity: warning
      - alert: DiskFull
        expr: disk_free < 0.05
        labels:
          severity: critical
`

// TestSymbolContextServesWorkloadFactsWithCoverage (WAE-5): symbol_context on
// a Kubernetes workload serves its K8sFact rows, and on a dialect entity its
// EntityFact rows, under `facts`, sorted by line then key, each anchored at
// its path and line, with coverage naming the extractor, tier and read set;
// a workload whose name prefixes another's serves only its own; a source
// changed since the index makes coverage incomplete and names the citation
// status on each anchor.
func TestSymbolContextServesWorkloadFactsWithCoverage(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "deploy/api.yaml"), entityFactsWorkload)
	writeFixtureFile(t, filepath.Join(root, "monitoring/rules.yml"), entityFactsRules)
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "facts.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: "facts", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	srv := mcp.NewServer(c)
	facts := func(sym string) ([]string, map[string]any) {
		t.Helper()
		res := callTool(t, srv, "symbol_context", map[string]any{"service": "facts", "symbol": sym, "snippet": false})
		f, ok := res["facts"].(map[string]any)
		if !ok {
			t.Fatalf("%s: no facts in %v", sym, res)
		}
		var rows []string
		for _, r := range f["rows"].([]any) {
			m := r.(map[string]any)
			a := m["anchor"].(map[string]any)
			row := fmt.Sprintf("%s:%v %s %s %s", a["path"], a["line"], m["kind"], m["key"], m["value"])
			if cs, ok := a["citation_status"].(map[string]any); ok {
				row += " [" + cs["state"].(string) + "]"
			}
			rows = append(rows, row)
		}
		return rows, f["coverage"].(map[string]any)
	}

	rows, cov := facts("Deployment/shop/api")
	want := []string{
		"deploy/api.yaml:5 K8sFact replicas 2",
		"deploy/api.yaml:10 K8sFact containers api",
		"deploy/api.yaml:15 K8sFact containers.api.resources.limits.memory 128Mi",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("facts(Deployment/shop/api)\n got %q\nwant %q", rows, want)
	}
	wantCov := map[string]any{"state": "complete", "extractor": "k8s", "tier": "verified", "read_set": []any{"deploy/api.yaml"}}
	if !reflect.DeepEqual(cov, wantCov) {
		t.Errorf("coverage = %v, want %v", cov, wantCov)
	}

	rows, cov = facts("NodeDown")
	want = []string{
		"monitoring/rules.yml:7 EntityFact labels.severity critical",
		"monitoring/rules.yml:8 EntityFact labels.team infra",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("facts(NodeDown)\n got %q\nwant %q", rows, want)
	}
	if cov["state"] != "complete" || cov["tier"] != "verified" || !reflect.DeepEqual(cov["read_set"], []any{"monitoring/rules.yml"}) {
		t.Errorf("coverage(NodeDown) = %v", cov)
	}

	// Two alerts in one file share a name (the warning and critical pair):
	// the bare name is ambiguous, the key resolves one of them, and that
	// def is served its own rows, never its namesake's (review F1).
	const diskFull = "facts::monitoring/rules.DiskFull"
	res := callTool(t, srv, "symbol_context", map[string]any{"service": "facts", "symbol": diskFull, "snippet": false})
	def, _ := res["def"].(map[string]any)
	if def == nil {
		t.Fatalf("DiskFull: no def in %v", res)
	}
	wantDisk := map[float64][]string{
		9:  {"monitoring/rules.yml:12 EntityFact labels.severity warning"},
		13: {"monitoring/rules.yml:16 EntityFact labels.severity critical"},
	}[def["line"].(float64)]
	if wantDisk == nil {
		t.Fatalf("DiskFull resolved to an unexpected def %v", def)
	}
	if rows, _ = facts(diskFull); !reflect.DeepEqual(rows, wantDisk) {
		t.Errorf("facts(DiskFull) at line %v\n got %q\nwant %q", def["line"], rows, wantDisk)
	}

	// The rule file changes after the index: the facts are still served,
	// but their coverage cannot be complete.
	if err := os.WriteFile(filepath.Join(root, "monitoring/rules.yml"), []byte(entityFactsRules+"      - alert: Extra\n        expr: vector(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, cov = facts("NodeDown")
	if cov["state"] != "incomplete" {
		t.Errorf("coverage after an edit = %v, want incomplete", cov)
	}
	for _, r := range rows {
		if r[len(r)-1] != ']' {
			t.Errorf("anchor after an edit names no citation status: %s", r)
		}
	}
}
