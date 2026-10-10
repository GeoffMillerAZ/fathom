package fusion

// grafana_ingest_test.go — the grafana-dashboard dialect through ingest
// (LB-F363, LB-F364, LB-F365; wave c3 item obs): a dashboard JSON is claimed
// by its content whatever its name and a JSON document that is no dashboard
// keeps the generic extractor; a panel's PromQL reaches the recording rule it
// reads, a templated metric is refused and a Loki query never scanned; a uid
// link reaches the dashboard, a datasource uid the provisioned datasource, a
// provider's container path the dashboards a compose bind mount puts there;
// an alerting provisioning file is Grafana's, never a Prometheus rule file;
// ReingestFile reads each file as ingest does.

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func grafanaRepo() map[string]string {
	return map[string]string{
		// a renamed dashboard: claimed by its content
		"grafana/dashboards/shop/checkout-v3.json": `{
  "uid": "checkout-overview",
  "title": "Checkout",
  "schemaVersion": 39,
  "templating": {"list": [{"name": "datasource", "type": "datasource", "query": "prometheus"}]},
  "links": [{"title": "Cart", "type": "link", "url": "/d/cart-overview/cart"}],
  "panels": [
    {"id": 1, "type": "timeseries", "title": "Rate",
     "datasource": {"type": "prometheus", "uid": "webstore-metrics"},
     "targets": [{"expr": "sum(job:http_requests:rate5m{job=~\"$job\"}) / sum(rate(http_requests_total[$__rate_interval]))"}]},
    {"id": 2, "type": "stat", "title": "Templated", "datasource": "$datasource",
     "targets": [{"expr": "$metric"}]},
    {"id": 3, "type": "logs", "title": "Logs", "datasource": {"type": "loki", "uid": "logs"},
     "targets": [{"expr": "{app=\"checkout\"} |= \"job:http_requests:rate5m\""}]}
  ]
}
`,
		"grafana/dashboards/shop/cart.json": `{"uid": "cart-overview", "title": "Cart", "schemaVersion": 39, "panels": []}
`,
		// no dashboard: the generic JSON extractor's
		"config/settings.json": `{"panels": ["a", "b"], "theme": "dark"}
`,
		"monitoring/recording.rules.yml": `groups:
  - name: http
    rules:
      - record: job:http_requests:rate5m
        expr: sum by (job) (rate(http_requests_total[5m]))
`,
		"grafana/provisioning/datasources/ds.yaml": `apiVersion: 1
datasources:
  - name: Prometheus
    uid: webstore-metrics
    type: prometheus
    url: http://prometheus:9090
`,
		"grafana/provisioning/dashboards/shop.yaml": `apiVersion: 1
providers:
  - name: shop
    type: file
    options:
      path: /var/lib/grafana/dashboards/shop
`,
		// a Grafana rule whose labels write an alert key: prometheus' sniff
		// would claim it if it were asked first
		"grafana/provisioning/alerting/rules.yaml": `apiVersion: 1
groups:
  - orgId: 1
    name: shop
    folder: Shop
    interval: 1m
    rules:
      - uid: r1
        title: CheckoutSlow
        condition: A
        labels:
          alert: page
        data:
          - refId: A
            datasourceUid: webstore-metrics
            model:
              datasource: {type: prometheus, uid: webstore-metrics}
              expr: job:http_requests:rate5m < 1
`,
		"docker-compose.yml": `services:
  grafana:
    image: grafana/grafana:12.0.0
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning
      - ./grafana/dashboards:/var/lib/grafana/dashboards
`,
	}
}

// TestIngest_GrafanaDashboardRepo (LB-F363, LB-F364, LB-F365).
func TestIngest_GrafanaDashboardRepo(t *testing.T) {
	p, st, dir := obsIngest(t, grafanaRepo(), "gf")
	kinds := crSourceKinds(t, st, "gf")
	for rel, want := range map[string][]string{
		"grafana/dashboards/shop/checkout-v3.json":  {"GrafanaDashboard", "GrafanaPanel", "GrafanaVariable", "GrafanaLink"},
		"grafana/dashboards/shop/cart.json":         {"GrafanaDashboard"},
		"grafana/provisioning/datasources/ds.yaml":  {"GrafanaProvisioning", "GrafanaDatasource"},
		"grafana/provisioning/dashboards/shop.yaml": {"GrafanaProvisioning", "GrafanaDashboardProvider"},
		"grafana/provisioning/alerting/rules.yaml":  {"GrafanaProvisioning", "GrafanaAlertRuleGroup", "GrafanaAlertRule"},
		"monitoring/recording.rules.yml":            {"PrometheusRecordingRule"},
	} {
		for _, k := range want {
			if !crHasKind(kinds[rel], k) {
				t.Errorf("%s read as %q, want %s among them", rel, kinds[rel], k)
			}
		}
	}
	if strings.Contains(kinds["config/settings.json"], "Grafana") {
		t.Errorf("a JSON document that is no dashboard read as %q", kinds["config/settings.json"])
	}
	if strings.Contains(kinds["grafana/provisioning/alerting/rules.yaml"], "Prometheus") {
		t.Errorf("an alerting provisioning file read as Prometheus: %q", kinds["grafana/provisioning/alerting/rules.yaml"])
	}
	for rel, want := range map[string]string{
		"grafana/dashboards/shop/checkout-v3.json": "grafana-dashboard",
		"grafana/provisioning/alerting/rules.yaml": "grafana-dashboard",
		"config/settings.json":                     "",
	} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if got := p.ContentDialect(rel, b, nil); got != want {
			t.Errorf("ContentDialect(%s) = %q, want %q", rel, got, want)
		}
	}

	got, _ := obsResolution(t, p, dir, "gf")
	want := []string{
		"grafana/dashboards/shop/checkout-v3.json|links[0] -> grafana/dashboards/shop/cart.json|cart-overview links_to",
		"grafana/dashboards/shop/checkout-v3.json|panel/1 -> grafana/provisioning/datasources/ds.yaml|webstore-metrics datasource",
		"grafana/dashboards/shop/checkout-v3.json|panel/1 -> monitoring/recording.rules.yml|job:http_requests:rate5m uses_record",
		"grafana/provisioning/alerting/rules.yaml|CheckoutSlow -> grafana/provisioning/datasources/ds.yaml|webstore-metrics datasource",
		"grafana/provisioning/alerting/rules.yaml|CheckoutSlow -> monitoring/recording.rules.yml|job:http_requests:rate5m uses_record",
		"grafana/provisioning/dashboards/shop.yaml|shop -> grafana/dashboards/shop/cart.json|cart-overview provides",
		"grafana/provisioning/dashboards/shop.yaml|shop -> grafana/dashboards/shop/checkout-v3.json|checkout-overview provides",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observability joins\n got %q\nwant %q", got, want)
	}
	edges := promEdges(t, st, "gf")
	for _, e := range []string{
		"grafana/dashboards/shop/checkout-v3.json|gf::grafana/dashboards/shop/checkout-v3.panel/1->gf::monitoring/recording.rules.job:http_requests:rate5m",
		"grafana/provisioning/dashboards/shop.yaml|gf::grafana/provisioning/dashboards/shop.shop->gf::grafana/dashboards/shop/cart.cart-overview",
	} {
		if !containsEdge(edges, e) {
			t.Errorf("missing stored edge %s\nstored: %q", e, edges)
		}
	}
}

// TestReingestFile_GrafanaDashboardRepo: the one-file refresh reads each
// file as ingest does.
func TestReingestFile_GrafanaDashboardRepo(t *testing.T) {
	p, st, dir := obsIngest(t, grafanaRepo(), "gf")
	reingestParity(t, p, st, dir, "gf", grafanaRepo())
}

// TestLateYAMLOrderingObs (LB-F365): the late-YAML hub asks grafana-dashboard
// and alertmanager before prometheus, so a Grafana alerting provisioning file
// whose rule writes an `alert:` key (which prometheus' sniff matches) is
// Grafana's; a Prometheus rule file and an Alertmanager configuration keep
// their own dialects.
func TestLateYAMLOrderingObs(t *testing.T) {
	grafanaRules := grafanaRepo()["grafana/provisioning/alerting/rules.yaml"]
	if x := lateYAMLLookup("prometheus"); x == nil || !x.DetectsContent([]byte(grafanaRules)) {
		t.Fatal("the fixture no longer meets prometheus' sniff: the ordering is not exercised")
	}
	for rel, want := range map[string]ToolKind{
		"grafana/provisioning/alerting/rules.yaml": ToolGrafanaDashboard,
		"monitoring/recording.rules.yml":           ToolPrometheus,
		"alerts/node.yaml":                         ToolPrometheus,
		"am/config.yaml":                           ToolAlertmanager,
		"am/alertmanager.yml":                      ToolAlertmanager,
	} {
		content := map[string]string{
			"grafana/provisioning/alerting/rules.yaml": grafanaRules,
			"monitoring/recording.rules.yml":           grafanaRepo()["monitoring/recording.rules.yml"],
			"alerts/node.yaml":                         "groups:\n  - name: n\n    rules:\n      - alert: NodeDown\n        expr: up == 0\n",
			"am/config.yaml":                           "route:\n  receiver: a\nreceivers:\n  - name: a\n",
			"am/alertmanager.yml":                      "global: {}\nroute: {receiver: a}\nreceivers: [{name: a}]\n",
		}[rel]
		if got := Classify(rel, []byte(content), nil); got != want {
			t.Errorf("Classify(%s) = %q, want %q", rel, got, want)
		}
	}
}
