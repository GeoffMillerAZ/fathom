package fusion

// k8sembed_ingest_test.go — documents Kubernetes objects embed, through
// ingest (LB-F366, wave c3 item obs): a ConfigMapList's dashboard, a
// GrafanaDashboard's spec.json, a Secret's alertmanager.yaml in stringData
// and as base64 data are read through their dialects beside the objects,
// their entities qualified by the key and placed at the manifest's lines;
// they join the Prometheus rules as a file of their own would; a Secret's
// other values are never read; ReingestFile reads each manifest as ingest
// does.

import (
	"context"
	"encoding/base64"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const embedSentinel = "EMBEDDED-SECRET-SENTINEL-4f1d"

func k8sEmbedRepo() map[string]string {
	b64 := base64.StdEncoding.EncodeToString([]byte("route:\n  receiver: b64-default\nreceivers:\n  - name: b64-default\n"))
	return map[string]string{
		"deploy/monitoring/grafana-dashboards.yaml": `apiVersion: v1
kind: ConfigMapList
items:
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: grafana-dashboard-cart
    namespace: monitoring
  data:
    cart.json: |-
      {
        "uid": "cart-k8s",
        "title": "Cart",
        "schemaVersion": 39,
        "panels": [
          {"id": 1, "type": "timeseries", "title": "Rate",
           "datasource": {"type": "prometheus", "uid": "$datasource"},
           "targets": [{"expr": "sum(job:http_requests:rate5m)"}]}
        ]
      }
    notes.txt: not a dashboard
`,
		"deploy/monitoring/grafana-operator.yaml": `apiVersion: grafana.integreatly.org/v1beta1
kind: GrafanaDashboard
metadata:
  name: operator-dash
spec:
  json: |
    {"uid": "operator-dash", "title": "Operator", "schemaVersion": 39,
     "links": [{"title": "Cart", "url": "/d/cart-k8s"}],
     "panels": []}
`,
		"deploy/monitoring/alertmanager-secret.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-main
  namespace: monitoring
stringData:
  alertmanager.yaml: |-
    route:
      receiver: default
      routes:
        - matchers: ['alertname = NodeDown']
          receiver: pager
    receivers:
      - name: default
      - name: pager
  slack-token: ` + embedSentinel + `
type: Opaque
`,
		"deploy/monitoring/alertmanager-b64.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-b64
data:
  alertmanager.yaml: ` + b64 + `
  password: ` + base64.StdEncoding.EncodeToString([]byte(embedSentinel)) + `
`,
		"monitoring/rules.yml": `groups:
  - name: node
    rules:
      - alert: NodeDown
        expr: up{job="node"} == 0
        labels:
          severity: critical
      - record: job:http_requests:rate5m
        expr: sum by (job) (rate(http_requests_total[5m]))
`,
	}
}

// TestIngest_K8sEmbeddedDocuments (LB-F366).
func TestIngest_K8sEmbeddedDocuments(t *testing.T) {
	p, st, dir := obsIngest(t, k8sEmbedRepo(), "k8s")
	obs, err := st.QueryObservations(context.Background(), "k8s", "")
	if err != nil {
		t.Fatal(err)
	}
	type at struct {
		kind string
		line int
	}
	byName := map[string]at{}
	for _, o := range obs {
		n := o.ToSymbolNode()
		byName[o.SourceRef+"|"+n.Name] = at{string(n.Kind), n.Line}
		for _, s := range []string{n.Name, n.Signature, strings.Join(n.Calls, ",")} {
			if strings.Contains(s, embedSentinel) {
				t.Errorf("a Secret's other value was stored: %+v", n)
			}
		}
	}
	for key, want := range map[string]at{
		"deploy/monitoring/grafana-dashboards.yaml|cart-k8s":                           {"GrafanaDashboard", 11},
		"deploy/monitoring/grafana-dashboards.yaml|cart.json/panel/1":                  {"GrafanaPanel", 16},
		"deploy/monitoring/grafana-operator.yaml|operator-dash":                        {"GrafanaDashboard", 7},
		"deploy/monitoring/grafana-operator.yaml|operator-dash/links[0]":               {"GrafanaLink", 8},
		"deploy/monitoring/alertmanager-secret.yaml|alertmanager.yaml":                 {"AlertmanagerConfig", 8},
		"deploy/monitoring/alertmanager-secret.yaml|alertmanager.yaml/route.routes[0]": {"AlertmanagerRoute", 11},
		"deploy/monitoring/alertmanager-secret.yaml|alertmanager.yaml/pager":           {"AlertmanagerReceiver", 15},
		"deploy/monitoring/alertmanager-b64.yaml|alertmanager.yaml/b64-default":        {"AlertmanagerReceiver", 6},
	} {
		if got := byName[key]; got != want {
			t.Errorf("%s = %+v, want %+v", key, got, want)
		}
	}
	for _, key := range []string{"deploy/monitoring/alertmanager-secret.yaml|Secret/monitoring/alertmanager-main", "deploy/monitoring/alertmanager-b64.yaml|Secret/alertmanager-b64", "deploy/monitoring/grafana-operator.yaml|GrafanaDashboard/operator-dash"} {
		if _, ok := byName[key]; !ok {
			t.Errorf("the Kubernetes object %s is no longer read beside its embedded document", key)
		}
	}

	got, _ := obsResolution(t, p, dir, "k8s")
	want := []string{
		"deploy/monitoring/alertmanager-secret.yaml|alertmanager.yaml/route.routes[0] -> monitoring/rules.yml|NodeDown alertname",
		"deploy/monitoring/grafana-dashboards.yaml|cart.json/panel/1 -> monitoring/rules.yml|job:http_requests:rate5m uses_record",
		"deploy/monitoring/grafana-operator.yaml|operator-dash/links[0] -> deploy/monitoring/grafana-dashboards.yaml|cart-k8s links_to",
		"monitoring/rules.yml|NodeDown -> deploy/monitoring/alertmanager-b64.yaml|alertmanager.yaml/b64-default routes_to",
		"monitoring/rules.yml|NodeDown -> deploy/monitoring/alertmanager-secret.yaml|alertmanager.yaml/pager routes_to",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observability joins\n got %q\nwant %q", got, want)
	}
	edges := promEdges(t, st, "k8s")
	for _, e := range []string{
		"monitoring/rules.yml|k8s::monitoring/rules.NodeDown->k8s::deploy/monitoring/alertmanager-secret.alertmanager.yaml/pager",
		"deploy/monitoring/grafana-dashboards.yaml|k8s::deploy/monitoring/grafana-dashboards.cart.json/panel/1->k8s::monitoring/rules.job:http_requests:rate5m",
	} {
		if !containsEdge(edges, e) {
			t.Errorf("missing stored edge %s\nstored: %q", e, edges)
		}
	}
}

// TestReingestFile_K8sEmbeddedDocuments: the one-file refresh reads each
// manifest, its embedded documents with it, as ingest does.
func TestReingestFile_K8sEmbeddedDocuments(t *testing.T) {
	p, st, dir := obsIngest(t, k8sEmbedRepo(), "k8s")
	reingestParity(t, p, st, dir, "k8s", k8sEmbedRepo())
}
