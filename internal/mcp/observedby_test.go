package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const observedByWorkloads = `apiVersion: apps/v1
kind: Deployment
metadata: {name: reviews, namespace: shop}
spec:
  selector: {matchLabels: {app: reviews}}
  template:
    metadata: {labels: {app: reviews, tier: web}}
    spec:
      containers: [{name: app, image: "acme/reviews:1"}]
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: lonely, namespace: shop}
spec:
  selector: {matchLabels: {app: lonely}}
  template:
    metadata: {labels: {app: lonely}}
    spec:
      containers: [{name: app, image: "acme/lonely:1"}]
---
apiVersion: v1
kind: Service
metadata: {name: reviews, namespace: shop, labels: {app: reviews}}
spec: {selector: {app: reviews}, ports: [{port: 80}]}
`

const observedByMonitors = `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata: {name: reviews-svc, namespace: monitoring}
spec:
  selector: {matchLabels: {app: reviews}}
  namespaceSelector: {matchNames: [shop]}
  endpoints: [{port: http}]
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata: {name: reviews-pods, namespace: monitoring}
spec:
  selector:
    matchExpressions:
      - {key: app, operator: In, values: [reviews, ratings]}
  namespaceSelector: {matchNames: [shop]}
  podMetricsEndpoints: [{port: http}]
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata: {name: nothing, namespace: monitoring}
spec:
  selector: {matchLabels: {app: nobody}}
  namespaceSelector: {matchNames: [shop]}
`

// TestSymbolContextNamesMonitorsSelectingWorkload (TL-15): symbol_context on
// a Kubernetes workload names the ServiceMonitor (through the Service whose
// selector reaches the workload) and PodMonitor objects whose selectors reach
// it, inside typed_edges; a monitor that selects nothing is absent and a
// workload no monitor selects names none.
func TestSymbolContextNamesMonitorsSelectingWorkload(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "deploy/workloads.yaml"), observedByWorkloads)
	writeFixtureFile(t, filepath.Join(root, "deploy/monitors.yaml"), observedByMonitors)
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "ob.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: "ob", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	srv := mcp.NewServer(c)
	observed := func(sym string) []map[string]any {
		t.Helper()
		res := callTool(t, srv, "symbol_context", map[string]any{"service": "ob", "symbol": sym, "snippet": false})
		te, ok := res["typed_edges"].(map[string]any)
		if !ok {
			t.Fatalf("%s: no typed_edges in %v", sym, res)
		}
		rows, ok := te["observed_by"].([]any)
		if !ok {
			t.Fatalf("%s: typed_edges has no observed_by: %v", sym, te)
		}
		var out []map[string]any
		for _, r := range rows {
			out = append(out, r.(map[string]any))
		}
		return out
	}
	got := observed("Deployment/shop/reviews")
	var names []string
	for _, r := range got {
		n := r["kind"].(string) + ":" + r["name"].(string)
		if via, _ := r["via"].(string); via != "" {
			n += " via " + via
		}
		names = append(names, n)
	}
	sort.Strings(names)
	want := []string{"PodMonitor:PodMonitor/monitoring/reviews-pods", "ServiceMonitor:ServiceMonitor/monitoring/reviews-svc via Service/shop/reviews"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Errorf("observed_by(reviews) = %q, want %q", names, want)
	}
	if lonely := observed("Deployment/shop/lonely"); len(lonely) != 0 {
		t.Errorf("observed_by(lonely) = %v, want none", lonely)
	}
}

func writeFixtureFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
