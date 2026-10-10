package fusion

// prometheus_ingest_test.go — the prometheus dialect through ingest
// (LB-F319, LB-F320; wave c1 item prom): the late-YAML hub claims a rule
// file, a configuration and a promtool test file by the manifest's names or
// the content sniff; ingest stores their entities, the record references a
// rule's expr makes, the rule_files a configuration or test names (doc-xref)
// and the route targets a scrape job's static target or a rule's identifying
// label names (route-ref, deploy: keys); ReingestFile reads each file as
// ingest does.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func prometheusRepo() map[string]string {
	return map[string]string{
		"monitoring/prometheus.yml": `global:
  scrape_interval: 15s
rule_files:
  - alerts.rules.yml
scrape_configs:
  - job_name: checkout
    static_configs:
      - targets: ["checkout:8080"]
  - job_name: node
    static_configs:
      - targets: ["localhost:9100"]
`,
		"monitoring/alerts.rules.yml": `groups:
  - name: checkout
    rules:
      - record: job:requests:rate5m
        expr: sum by (job) (rate(http_requests_total{job="checkout"}[5m]))
      - alert: CheckoutSlow
        expr: job:requests:rate5m < 1
      - alert: CartDown
        expr: up{job="cart", namespace="shop"} == 0
      - alert: AnyDown
        expr: up{job=~".+"} == 0
`,
		// claimed by content alone (no manifest name): read, its route
		// targets not part of this test (route candidacy is path-only)
		"monitoring/more.yaml": `groups:
  - name: more
    rules:
      - alert: CheckoutAbsent
        expr: absent(up{job="checkout"})
`,
		"monitoring/tests.yml": `rule_files:
  - alerts.rules.yml
tests:
  - name: slow
    interval: 1m
    alert_rule_test:
      - alertname: CheckoutSlow
        eval_time: 5m
`,
		"docker-compose.yml": `services:
  checkout:
    image: shop/checkout:1
  prometheus:
    image: prom/prometheus:v3.15.0
`,
		"deploy/k8s/cart.yaml": `apiVersion: v1
kind: Service
metadata:
  name: cart
  namespace: shop
spec:
  selector:
    app: cart
`,
	}
}

func prometheusIngest(t *testing.T) (*Pipeline, store.Store, string, Report) {
	t.Helper()
	dir := t.TempDir()
	writeCorpus(t, dir, prometheusRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "prom", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	return p, st, dir, rep
}

func promEdges(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	all, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(got)
	return got
}

// TestIngest_PrometheusRepo (LB-F319): ingest reads every Prometheus file
// with the dialect, stores a rule's record references in its calls, and
// joins a configuration's and a test file's rule_files to the rule file.
func TestIngest_PrometheusRepo(t *testing.T) {
	_, st, _, _ := prometheusIngest(t)
	kinds := crSourceKinds(t, st, "prom")
	for rel, want := range map[string][]string{
		"monitoring/prometheus.yml":   {"PrometheusFile", "PrometheusScrapeJob", "TelemetryIdentity"},
		"monitoring/alerts.rules.yml": {"PrometheusFile", "PrometheusRuleGroup", "PrometheusRecordingRule", "PrometheusAlertingRule"},
		"monitoring/more.yaml":        {"PrometheusFile", "PrometheusRuleGroup", "PrometheusAlertingRule"},
		"monitoring/tests.yml":        {"PrometheusFile", "PromtoolTest"},
	} {
		for _, k := range want {
			if !crHasKind(kinds[rel], k) {
				t.Errorf("%s read as %q, want %s among them", rel, kinds[rel], k)
			}
		}
	}
	obs, err := st.QueryObservations(context.Background(), "prom", "")
	if err != nil {
		t.Fatal(err)
	}
	var sig, calls string
	for _, o := range obs {
		n := o.ToSymbolNode()
		switch {
		case n.Kind == "TelemetryIdentity" && n.Name == "checkout":
			sig = n.Signature
		case n.Kind == "PrometheusAlertingRule" && n.Name == "CheckoutSlow":
			calls = strings.Join(n.Calls, ",")
		}
	}
	if sig != "prometheus.job=checkout" {
		t.Errorf("TelemetryIdentity checkout signature = %q, want prometheus.job=checkout", sig)
	}
	if calls != "job:requests:rate5m" {
		t.Errorf("CheckoutSlow calls = %q, want the recording rule it reads", calls)
	}
	edges := promEdges(t, st, "prom")
	for _, want := range []string{
		"monitoring/prometheus.yml|prom::monitoring/prometheus.prometheus.yml->prom::monitoring/alerts.rules.alerts.rules.yml",
		"monitoring/tests.yml|prom::monitoring/tests.tests.yml->prom::monitoring/alerts.rules.alerts.rules.yml",
	} {
		if !containsEdge(edges, want) {
			t.Errorf("missing rule_files edge %s\nstored: %q", want, edges)
		}
	}
}

// TestIngest_PrometheusRouteRefs (LB-F320): a scrape job's static target and
// a rule's identifying-label equality matcher reach the route pass for a file
// the manifest claims by name, and join the compose service or Kubernetes
// Service whose deploy: key they name; a regex matcher and an IP literal join
// nothing.
func TestIngest_PrometheusRouteRefs(t *testing.T) {
	_, st, _, rep := prometheusIngest(t)
	var got []string
	for _, e := range promEdges(t, st, "prom") {
		if strings.HasPrefix(e, "monitoring/prometheus.yml|") || strings.HasPrefix(e, "monitoring/alerts.rules.yml|") {
			if strings.Contains(e, "->prom::docker-compose.") || strings.Contains(e, "->prom::deploy/k8s/") {
				got = append(got, e)
			}
		}
	}
	want := []string{
		"monitoring/alerts.rules.yml|prom::monitoring/alerts.rules.CartDown->prom::deploy/k8s/cart.Service/shop/cart",
		"monitoring/alerts.rules.yml|prom::monitoring/alerts.rules.job:requests:rate5m->prom::docker-compose.checkout",
		"monitoring/prometheus.yml|prom::monitoring/prometheus.checkout->prom::docker-compose.checkout",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("route edges from the name-claimed Prometheus files\n got: %q\nwant: %q", got, want)
	}
	if rep.RouteRefUnresolved[RouteIPLiteral] < 1 {
		t.Errorf("RouteRefUnresolved = %v, want the localhost target counted ip-literal", rep.RouteRefUnresolved)
	}
}

// TestReingestFile_PrometheusRepo: the one-file refresh reads each
// Prometheus file as ingest's dispatch does.
func TestReingestFile_PrometheusRepo(t *testing.T) {
	p, st, dir, _ := prometheusIngest(t)
	before := crSourceKinds(t, st, "prom")
	for rel := range prometheusRepo() {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("# edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(context.Background(), st, "prom", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	if after := crSourceKinds(t, st, "prom"); !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}

func containsEdge(edges []string, want string) bool {
	for _, e := range edges {
		if e == want {
			return true
		}
	}
	return false
}
